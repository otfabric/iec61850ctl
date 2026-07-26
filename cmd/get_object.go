// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	iec61850 "github.com/otfabric/go-iec61850"
	"github.com/otfabric/iec61850ctl/internal/app"
	"github.com/otfabric/iec61850ctl/pkg/domain"
	"github.com/otfabric/iec61850ctl/pkg/formatter"
	"github.com/otfabric/iec61850ctl/pkg/service"
	"github.com/otfabric/iec61850ctl/pkg/stack/client"
)

var (
	object             string
	fcString           string
	valType            string
	objectDetailedFlag bool
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get/read values from the IEC 61850 server",
}

var getObjectCmd = &cobra.Command{
	Use:   "object",
	Short: "Read a specific data object from the IEC 61850 server",
	Long: `Read the value of a specific data object using its object reference.
The object reference format is: LogicalDevice/LogicalNode.DataObject.attribute
Example: DEV03LD0/CUS1_GGIO3.AnIn1.mag.f`,
	RunE: runGetObject,
}

func init() {
	getObjectCmd.Flags().StringVar(&object, "object", "", "object reference to read (required)")
	getObjectCmd.Flags().StringVar(&fcString, "fc", "MX", "functional constraint (ST, MX, SP, SV, CF, DC, SG, SE, SR, OR, BL, EX, CO, RP, BR, ALL)")
	getObjectCmd.Flags().StringVar(&valType, "type", "???", "value type hint (unused; type is inferred from the server)")
	getObjectCmd.Flags().BoolVar(&objectDetailedFlag, "detailed", false, "show detailed information including FC and Type")
	_ = getObjectCmd.MarkFlagRequired("object")

	getCmd.AddCommand(getObjectCmd)
	rootCmd.AddCommand(getCmd)
}

func runGetObject(cmd *cobra.Command, args []string) error {
	fcModel := domain.ParseFC(fcString)
	if !fcModel.IsValid() {
		return fmt.Errorf("invalid functional constraint: %s", fcString)
	}

	finalHost, finalPort, err := getHostPort()
	if err != nil {
		return err
	}
	printConnectionTarget(finalHost, finalPort)

	conn, err := client.NewConnection(client.ConnectionInput{
		Host:           finalHost,
		Port:           finalPort,
		ConnectTimeout: 10,
		RequestTimeout: 10,
	})
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(context.Background()) }()

	a := app.New(conn)
	obj, err := a.GetObject(app.GetObjectInput{Object: object, FC: fcModel})
	if err != nil {
		return err
	}

	if !objectDetailedFlag {
		if obj.Value != nil {
			fmt.Printf("Value: %s\n", obj.Value.String())
		} else {
			fmt.Println("Value: <nil>")
		}
		return nil
	}

	refStr := object
	if !strings.HasSuffix(object, "]") {
		refStr = object + "[" + string(fcModel) + "]"
	}
	ref, err := iec61850.ParseRef(refStr)
	if err != nil {
		return err
	}
	spec, specErr := conn.GetVariableType(context.Background(), ref)
	typeStr := ""
	if specErr == nil && spec != nil {
		typeStr = formatter.FormatTypeSpec(spec)
	}

	fmt.Printf("Object: %s\n", object)
	fmt.Printf("FC: %s\n", fcModel.String())
	if typeStr != "" {
		fmt.Printf("Type: %s\n", typeStr)
	}

	if obj.Type == domain.TypeStructure || strings.Contains(strings.ToLower(typeStr), "structure") {
		ld, ln, doName := parseObjectRef(object)
		if ld != "" && ln != "" && doName != "" {
			attrsMap, listErr := service.NewExplorer(conn).ListDataAttributes(service.ListDataAttributesInput{
				LogicalDevice: ld,
				LogicalNode:   ln,
				DataObject:    doName,
				Detailed:      true,
			})
			if listErr == nil {
				fmt.Println("Value:")
				lastSegment := ""
				if lastDot := strings.LastIndex(object, "."); lastDot >= 0 {
					lastSegment = object[lastDot+1:]
				}
				parentNames := make([]string, 0, len(attrsMap))
				if lastSegment != "" {
					if _, has := attrsMap[lastSegment]; has {
						parentNames = []string{lastSegment}
					}
				}
				if len(parentNames) == 0 {
					for k := range attrsMap {
						parentNames = append(parentNames, k)
					}
					sort.Strings(parentNames)
				}
				for _, parentName := range parentNames {
					for _, attr := range attrsMap[parentName] {
						daPath := buildStructAttrPath(object, parentName, attr.Name)
						fcAttr := "?"
						if attr.FC != "" {
							fcAttr = attr.FC.String()
						}
						valueStr := formatter.FormatDataAttributeValue(&attr)
						fmt.Printf("  %s [FC=%s] Type: %s Value: %s\n", daPath, fcAttr, attr.Type.String(), valueStr)
					}
				}
				return nil
			}
		}
	}

	if obj.Value != nil {
		fmt.Printf("Value: %s\n", obj.Value.String())
	} else {
		fmt.Println("Value: <nil>")
	}
	return nil
}

func buildStructAttrPath(object, parentName, attrName string) string {
	if parentName == "" {
		if attrName != "" {
			return object + "." + attrName
		}
		return object
	}
	lastDot := strings.LastIndex(object, ".")
	if lastDot >= 0 && object[lastDot+1:] == parentName {
		if attrName != "" && attrName != parentName {
			if strings.HasPrefix(attrName, parentName+".") {
				return object + "." + attrName
			}
			return object + "." + attrName
		}
		return object
	}
	daPath := object + "." + parentName
	if attrName != "" && attrName != parentName {
		if strings.HasPrefix(attrName, parentName+".") {
			return object + "." + attrName
		}
		daPath += "." + attrName
	}
	return daPath
}

func parseObjectRef(object string) (ld, ln, doName string) {
	slash := strings.Index(object, "/")
	if slash < 0 {
		return "", "", ""
	}
	ld = object[:slash]
	rest := object[slash+1:]
	ln, doName, ok := strings.Cut(rest, ".")
	if !ok {
		return ld, rest, ""
	}
	return ld, ln, doName
}
