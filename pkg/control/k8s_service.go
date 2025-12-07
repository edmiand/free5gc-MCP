// Package control provides Kubernetes/Helm configuration management utilities.
//
// k8s_servuce.go (Kubernetes Service Configuration) contains functions for:
//   - YAML node tree traversal and modification
//   - Dynamic SMF configuration updates (CIDR modifications in values.yaml)
//   - Dynamic UPF configuration updates (IP pool and routing rule modifications)
//   - Validation and manipulation of free5gc-helm configuration files
//
// These functions support the update_nfconfig MCP tool, enabling dynamic
// network function configuration without manual YAML editing or cluster restarts.
//
// Note: This file works with the free5gc-helm repository structure:
//   - free5gc-helm/charts/free5gc/charts/free5gc-smf/values.yaml
//   - free5gc-helm/charts/free5gc/charts/free5gc-upf/values.yaml
package control

import (
	"bytes"
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// FindNode : according path to travel yaml.Node tree
// support sting map and index string ("0")
func FindNode(root *yaml.Node, path []string) (*yaml.Node, error) {
	current := root

	// if it is Document Node => directly access
	if current.Kind == yaml.DocumentNode {
		if len(current.Content) == 0 {
			return nil, fmt.Errorf("empty yaml document")
		}
		current = current.Content[0]
	}

	for _, key := range path {
		found := false

		if current.Kind == yaml.MappingNode {
			// Mapping Node's Content is [Key, Value, Key, Value...]
			for i := 0; i < len(current.Content); i += 2 {
				keyNode := current.Content[i]
				valNode := current.Content[i+1]
				if keyNode.Value == key {
					current = valNode
					found = true
					break
				}
			}
		} else if current.Kind == yaml.SequenceNode {
			// Sequence Node's Content is [Item, Item...]
			// try to change key to number indexing 
			index, err := strconv.Atoi(key)
			if err != nil {
				return nil, fmt.Errorf("expected numeric index for sequence, got '%s'", key)
			}
			if index < 0 || index >= len(current.Content) {
				return nil, fmt.Errorf("index %d out of bounds", index)
			}
			current = current.Content[index]
			found = true
		} else {
			return nil, fmt.Errorf("unexpected node kind %v when looking for '%s'", current.Kind, key)
		}

		if !found {
			return nil, fmt.Errorf("key '%s' not found", key)
		}
	}

	return current, nil
}

// ModifySMFValuesYaml : modify cidr in SMF values.yaml
func ModifySMFValuesYaml(filePath string, targetUPFName string, newCidr string) error {
	// 1. read YAML file (values.yaml)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file error: %w", err)
	}

	var rootNode yaml.Node
	if err := yaml.Unmarshal(data, &rootNode); err != nil {
		return fmt.Errorf("unmarshal values.yaml error: %w", err)
	}

	// locate to smf.configuration.configuration ( format in free5gc-smf/values.yaml is a large string )
	outerPath := []string{"smf", "configuration", "configuration"}
	configNode, err := FindNode(&rootNode, outerPath)
	if err != nil {
		return fmt.Errorf("failed to find external configuration block: %w", err)
	}

	// check is string node or not
	if configNode.Kind != yaml.ScalarNode {
		return fmt.Errorf("configuration node is not a scalar string")
	}

	// 2. for Configuration String YAML file (values.yaml)
	var innerRootNode yaml.Node
	if err := yaml.Unmarshal([]byte(configNode.Value), &innerRootNode); err != nil {
		return fmt.Errorf("unmarshal inner configuration string error: %w", err)
	}

	// construct inner path to cidr position
	// path : userplaneInformation -> upNodes -> [AnchorUPF1] -> sNssaiUpfInfos -> 0 -> dnnUpfInfoList -> 0 -> pools -> 0 -> cidr
	innerPath := []string{
		"userplaneInformation",
		"upNodes",
		targetUPFName,
		"sNssaiUpfInfos",
		"0", // index
		"dnnUpfInfoList",
		"0",
		"pools",
		"0",
		"cidr",
	}

	// 3. find CIDR node to modify
	cidrNode, err := FindNode(&innerRootNode, innerPath)
	if err != nil {
		return fmt.Errorf("failed to find CIDR node for %s: %w", targetUPFName, err)
	}
	cidrNode.Value = newCidr 

	// 4. write back
	// node -> string ( need to remain the sequence of original file)
	var innerBuf bytes.Buffer
	innerEncoder := yaml.NewEncoder(&innerBuf)
	innerEncoder.SetIndent(2)
	if err := innerEncoder.Encode(innerRootNode.Content[0]); err != nil {
		return fmt.Errorf("encode inner yaml error: %w", err)
	}
	
	// write configuration string back to smf node
	configNode.Value = innerBuf.String()

	// yaml.LiteralStyle = |
	configNode.Style = yaml.LiteralStyle

	var outerBuf bytes.Buffer
	outerEncoder := yaml.NewEncoder(&outerBuf)
	outerEncoder.SetIndent(2)
	if err := outerEncoder.Encode(&rootNode); err != nil {
		return fmt.Errorf("encode outer yaml error: %w", err)
	}

	if err := os.WriteFile(filePath, outerBuf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write file error: %w", err)
	}

	return nil
}

// ModifyUPFValuesYaml : modify CIDR in UPF values.yaml
func ModifyUPFValuesYaml(filePath string, rootKey string, newCidr string) error {
	// 1. read YAML file (values.yaml)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read UPF file error: %w", err)
	}

	var rootNode yaml.Node
	if err := yaml.Unmarshal(data, &rootNode); err != nil {
		return fmt.Errorf("unmarshal UPF values error: %w", err)
	}

	// 2. consturct path ：[rootKey] -> configuration -> dnnList -> 0 -> cidr
	targetPath := []string{
		rootKey,          // "upf1"
		"configuration",
		"dnnList",
		"0", 
		"cidr",
	}

	// 3. find the node to modify
	cidrNode, err := FindNode(&rootNode, targetPath)
	if err != nil {
		return fmt.Errorf("failed to find CIDR node in UPF (%s): %w", rootKey, err)
	}
	
	// renew
	cidrNode.Value = newCidr

	// 4. write back
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&rootNode); err != nil {
		return fmt.Errorf("encode UPF yaml error: %w", err)
	}

	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write UPF file error: %w", err)
	}

	return nil
}