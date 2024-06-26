package fileparser

import (
	"fmt"
	"strings"
)

const (
	base = 10
)

// DataFileMessage represents a message in the data file, this implementation specifically
// written for .bin files for Mavlink telemetry file messages. Testing needs to be added for
// .log, .tlog files etc.
type DataFileMessage struct {
	Format          *DataFileFormat
	Elements        []interface{}
	ApplyMultiplier bool
	FieldNames      []string
	Parent          *BinaryDataFileReader
}

func NewDFMessage(dataFormat *DataFileFormat, elements []interface{}, applyMultiplier bool, reader *BinaryDataFileReader) *DataFileMessage {
	return &DataFileMessage{
		Format:          dataFormat,
		Elements:        elements,
		ApplyMultiplier: applyMultiplier,
		FieldNames:      dataFormat.Columns,
		Parent:          reader,
	}
}

func (dataMessage *DataFileMessage) ToMap() map[string]interface{} {
	dataMap := make(map[string]interface{})
	dataMap["mavpackettype"] = dataMessage.Format.Name

	for _, field := range dataMessage.FieldNames {
		// In this loop, each iteration is adding a new key-value pair to the map
		if _, exists := dataMap[field]; !exists {
			dataMap[field], _ = dataMessage.GetAttribute(field)
		}
	}

	return dataMap
}

// retrieves an attribute from the message
func (dataMessage *DataFileMessage) GetAttribute(field string) (interface{}, error) {
	index, ok := dataMessage.Format.ColumnHash[field]
	if !ok {
		return nil, fmt.Errorf("attribute %s not found", field)
	}

	value := dataMessage.Elements[index]

	// Handle specific cases based on format and message type
	if dataMessage.Format.MessageFormats[index] == "Z" && dataMessage.Format.Name == "FILE" {
		return value, nil
	}

	// Convert []byte to string if necessary
	if bytesValue, ok := value.([]byte); ok {
		value = string(bytesValue)
	}

	// Apply type-specific functions or multipliers
	if dataMessage.Format.Format[index] != 'M' || dataMessage.ApplyMultiplier {
		value = applyTypeFunctions(value, dataMessage.Format.MessageTypes[index])
	}

	// Convert to string with null termination
	if stringValue, ok := value.(string); ok {
		value = nullTerm(stringValue)
	}

	return value, nil
}

// applies type-specific functions or multipliers to a value
func applyTypeFunctions(value interface{}, messageType interface{}) interface{} {
	switch msgType := messageType.(type) {
	case func(interface{}) interface{}:
		return msgType(value)
	case uint32:
		switch v := value.(type) {
		case float64:
			return v * float64(msgType)
		case uint32:
			if msgType != 0 {
				return v * msgType
			}
		case int:
			return v
		}
	default:
		// Handle other types or return value as is
	}

	return value
}

// truncates a string at the first null character
func nullTerm(s string) string {
	idx := strings.Index(s, "\x00")
	if idx != -1 {
		s = s[:idx]
	}
	return s
}

// It sets the value in the Elements slice at the correct index.
// The function allows setting a specific field in the message.
// It includes error checking for non-existent fields.
// It applies a multiplier (actually a divider here) if specified and enabled.
// It directly modifies the Elements slice of the message.
// returns nil if successful (no error)
func (dataMessage *DataFileMessage) SetAttribute(field string, value interface{}) error {
	i, ok := dataMessage.Format.ColumnHash[field]
	if !ok {
		return fmt.Errorf("AttributeError: " + field)
	}

	if dataMessage.Format.MessageMults[i] != nil && dataMessage.ApplyMultiplier {
		value = value.(float64) / dataMessage.Format.MessageMults[i].(float64)
	}

	dataMessage.Elements[i] = value
	return nil
}

func (dataMessage *DataFileMessage) GetType() string {
	return dataMessage.Format.Name
}

func (dataMessage *DataFileMessage) GetMessage() string {
	for i, field := range dataMessage.FieldNames {
		if field == "Message" {
			// Check if the element is of type []uint8
			if byteArray, ok := dataMessage.Elements[i].([]uint8); ok {
				// Convert the []uint8 to a string
				str := string(byteArray)
				// Trim the string at the first null character
				str = nullTerm(str)
				return str
			}
		}
	}
	return ""
}

func (dataMessage *DataFileMessage) GetMode() int {
	for i, field := range dataMessage.FieldNames {
		if field == "Mode" {
			return dataMessage.Elements[i].(int)
		}

		if field == "ModeNum" {
			return dataMessage.Elements[i].(int)
		}
	}
	return -1
}
