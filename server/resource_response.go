package server

import (
	"encoding/json"
	"fmt"
)

// FormatResourceResponse formats a response according to MCP validation requirements.
// This ensures that text/blob content items have the required fields and format.
func FormatResourceResponse(uri string, result interface{}, version string) map[string]interface{} {
	// First check if result implements ResourceConverter
	if converter, ok := result.(ResourceConverter); ok {
		result = converter.ToResourceResponse()
	}

	// Handle specialized resource types
	switch v := result.(type) {
	case TextResource:
		result = v.ToResourceResponse()
	case ImageResource:
		result = v.ToResourceResponse()
	case LinkResource:
		result = v.ToResourceResponse()
	case FileResource:
		result = v.ToResourceResponse()
	case JSONResource:
		result = v.ToResourceResponse()
	case AudioResource:
		result = v.ToResourceResponse()
	}

	// Handle different versions with appropriate format
	switch version {
	case "2024-11-05":
		return formatResourceV20241105(uri, result)
	case "2025-03-26":
		return formatResourceV20250326(uri, result)
	case "2025-06-18":
		return formatResourceV20250618(uri, result)
	case "draft":
		// For now, draft has the same format as 2025-06-18
		// In the future, if they diverge, implement a separate formatter
		return formatResourceV20250618(uri, result)
	default:
		// If version is unknown, use the most recent format
		return formatResourceV20250618(uri, result)
	}
}

// formatResourceV20241105 formats a response for the 2024-11-05 MCP specification
func formatResourceV20241105(uri string, result interface{}) map[string]interface{} {
	// Special handling for string results
	if str, ok := result.(string); ok {
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": str,
				},
			},
		}
	}

	// Handle map results, ensuring proper structure
	if resultMap, ok := result.(map[string]interface{}); ok {
		// If it already has a content field, ensure it's properly formatted
		if content, hasContent := resultMap["content"]; hasContent {
			// Special case: explicitly empty content array
			if contentArr, isArray := content.([]interface{}); isArray && len(contentArr) == 0 {
				// Preserve empty array as is
				resultMap["content"] = []interface{}{}
				return resultMap
			}

			contentArray := ensureArray(content)
			validContent := make([]map[string]interface{}, 0, len(contentArray))

			// Validate each content item
			for _, item := range contentArray {
				if contentItem, ok := item.(map[string]interface{}); ok {
					// Ensure type is present
					contentType, _ := contentItem["type"].(string)
					if contentType == "" {
						contentType = "text"
						contentItem["type"] = "text"
					}

					// Ensure required fields for each content type
					switch contentType {
					case "text":
						if _, hasText := contentItem["text"].(string); !hasText {
							contentItem["text"] = "Default text content"
						}
					case "image":
						if _, hasURL := contentItem["imageUrl"].(string); hasURL {
							// Keep as image content type with required fields
							if _, hasAlt := contentItem["altText"].(string); !hasAlt {
								contentItem["altText"] = "Image"
							}
						} else {
							// Convert to text if missing required URL field
							contentItem["type"] = "text"
							contentItem["text"] = "Image URL missing"
						}
					case "link":
						if _, hasURL := contentItem["url"].(string); hasURL {
							// Keep as link content type with required fields
							if _, hasTitle := contentItem["title"].(string); !hasTitle {
								contentItem["title"] = "Link"
							}
						} else {
							// Convert to text if missing required URL field
							contentItem["type"] = "text"
							contentItem["text"] = "Link URL missing"
						}
					case "audio":
						// Convert audio to link in 2024-11-05 (which doesn't support audio)
						if audioUrl, hasUrl := contentItem["audioUrl"].(string); hasUrl {
							// Convert to link type
							contentItem["type"] = "link"
							contentItem["url"] = audioUrl

							// Use altText as title if available, otherwise generic title
							if altText, hasAlt := contentItem["altText"].(string); hasAlt && altText != "" {
								contentItem["title"] = altText
							} else if mimeType, hasMime := contentItem["mimeType"].(string); hasMime {
								contentItem["title"] = "Audio file: " + mimeType
							} else {
								contentItem["title"] = "Audio file"
							}

							// Remove audio-specific fields
							delete(contentItem, "audioUrl")
							delete(contentItem, "data")
							delete(contentItem, "mimeType")
							delete(contentItem, "altText")
						} else {
							// Convert to text if no URL available
							contentItem["type"] = "text"
							contentItem["text"] = "Audio file (no URL available)"
						}
					case "blob":
						if _, hasBlob := contentItem["blob"].(string); !hasBlob {
							contentItem["blob"] = "Default blob content"
						}
						if _, hasMimeType := contentItem["mimeType"].(string); !hasMimeType {
							contentItem["mimeType"] = "application/octet-stream"
						}
					default:
						// Unknown type, ensure it has a text field
						if _, hasText := contentItem["text"].(string); !hasText {
							contentItem["text"] = fmt.Sprintf("Content of type: %s", contentType)
						}
					}

					validContent = append(validContent, contentItem)
				} else {
					// Convert non-map items to text
					validContent = append(validContent, map[string]interface{}{
						"type": "text",
						"text": fmt.Sprintf("%v", item),
					})
				}
			}

			// If no content items, add a default one
			if len(validContent) == 0 {
				validContent = append(validContent, map[string]interface{}{
					"type": "text",
					"text": "Default content",
				})
			}

			resultMap["content"] = validContent
			return resultMap
		}

		// Handle contents array (2025-03-26 format) and convert to 2024-11-05 format
		if contents, hasContents := resultMap["contents"]; hasContents {
			contentsArray := ensureArray(contents)

			// Create a content array from the contents structure
			allContent := make([]map[string]interface{}, 0)

			// Process each content item in the contents array
			for _, item := range contentsArray {
				if contentsItem, ok := item.(map[string]interface{}); ok {
					// Extract inner content array
					if innerContent, hasInnerContent := contentsItem["content"]; hasInnerContent {
						// Special case: Check for explicit empty array
						if innerArr, isArray := innerContent.([]interface{}); isArray && len(innerArr) == 0 {
							// For empty content array, return empty content array in 2024-11-05 format
							return map[string]interface{}{
								"content": []interface{}{},
							}
						}

						innerContentArray := ensureArray(innerContent)

						// Add each inner content item to the flattened content array
						for _, innerItem := range innerContentArray {
							if innerItemMap, ok := innerItem.(map[string]interface{}); ok {
								allContent = append(allContent, innerItemMap)
							} else {
								// Convert non-map items to text
								allContent = append(allContent, map[string]interface{}{
									"type": "text",
									"text": fmt.Sprintf("%v", innerItem),
								})
							}
						}
					} else {
						// No inner content, use text or blob field if available
						if text, hasText := contentsItem["text"].(string); hasText {
							allContent = append(allContent, map[string]interface{}{
								"type": "text",
								"text": text,
							})
						} else if blob, hasBlob := contentsItem["blob"].(string); hasBlob {
							blobItem := map[string]interface{}{
								"type": "blob",
								"blob": blob,
							}
							if mimeType, hasMimeType := contentsItem["mimeType"].(string); hasMimeType {
								blobItem["mimeType"] = mimeType
							} else {
								blobItem["mimeType"] = "application/octet-stream"
							}
							allContent = append(allContent, blobItem)
						} else {
							// Default text content
							allContent = append(allContent, map[string]interface{}{
								"type": "text",
								"text": "Default content",
							})
						}
					}
				}
			}

			// If no content items after processing, preserve empty array
			if len(allContent) == 0 {
				return map[string]interface{}{
					"content": []interface{}{},
				}
			}

			// Create the result in 2024-11-05 format
			result := map[string]interface{}{
				"content": allContent,
			}

			// Copy metadata if present
			if metadata, hasMetadata := resultMap["metadata"]; hasMetadata {
				result["metadata"] = metadata
			}

			return result
		}

		// No content array, create a default one
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "Default content",
				},
			},
		}
	}

	// For any other type, convert to JSON string and format as text
	jsonData, err := json.Marshal(result)
	if err != nil {
		return formatResourceV20241105(uri, fmt.Sprintf("%v", result))
	}
	return formatResourceV20241105(uri, string(jsonData))
}

// formatResourceV20250326 formats a response for the 2025-03-26 MCP specification
func formatResourceV20250326(uri string, result interface{}) map[string]interface{} {
	// Handle specialized resource types first
	switch v := result.(type) {
	case TextResource:
		response := v.ToResourceResponse()
		return ensureContentsArray(response, uri)
	case ImageResource:
		response := v.ToResourceResponse()
		return ensureContentsArray(response, uri)
	case LinkResource:
		response := v.ToResourceResponse()
		return ensureContentsArray(response, uri)
	case FileResource:
		response := v.ToResourceResponse()
		return ensureContentsArray(response, uri)
	case JSONResource:
		response := v.ToResourceResponse()
		return ensureContentsArray(response, uri)
	case AudioResource:
		response := v.ToResourceResponse()
		return ensureContentsArray(response, uri)
	}

	// Handle different result types
	switch v := result.(type) {
	case string:
		// Simple text content
		response := map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": v,
				},
			},
		}
		return ensureContentsArray(response, uri)

	case map[string]interface{}:
		// If it already has proper structure, ensure contents array format
		if _, hasContents := v["contents"]; hasContents {
			return ensureContentsArray(v, uri)
		}
		if _, hasContent := v["content"]; hasContent {
			return ensureContentsArray(v, uri)
		}

		// For other maps, preserve them as-is and let ensureContentsArray handle the conversion
		return ensureContentsArray(v, uri)

	default:
		// Convert other types to JSON text
		jsonStr, _ := json.MarshalIndent(v, "", "  ")
		response := map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": string(jsonStr),
				},
			},
		}
		return ensureContentsArray(response, uri)
	}
}

// formatResourceV20250618 formats a response for the 2025-06-18 MCP specification
func formatResourceV20250618(uri string, result interface{}) map[string]interface{} {
	// Handle specialized resource types first
	switch v := result.(type) {
	case TextResource:
		response := v.ToResourceResponse()
		return ensureContentsArray(response, uri)
	case ImageResource:
		response := v.ToResourceResponse()
		return ensureContentsArray(response, uri)
	case LinkResource:
		response := v.ToResourceResponse()
		return ensureContentsArray(response, uri)
	case FileResource:
		response := v.ToResourceResponse()
		return ensureContentsArray(response, uri)
	case JSONResource:
		response := v.ToResourceResponse()
		return ensureContentsArray(response, uri)
	case AudioResource:
		response := v.ToResourceResponse()
		return ensureContentsArray(response, uri)
	}

	// Handle different result types
	switch v := result.(type) {
	case string:
		// Simple text content
		response := map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": v,
				},
			},
		}
		return ensureContentsArray(response, uri)

	case map[string]interface{}:
		// If it already has proper structure, ensure contents array format
		if _, hasContents := v["contents"]; hasContents {
			return ensureContentsArray(v, uri)
		}
		if _, hasContent := v["content"]; hasContent {
			return ensureContentsArray(v, uri)
		}

		// For other maps, preserve them as-is and let ensureContentsArray handle the conversion
		return ensureContentsArray(v, uri)

	default:
		// Convert other types to JSON text
		jsonStr, _ := json.MarshalIndent(v, "", "  ")
		response := map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": string(jsonStr),
				},
			},
		}
		return ensureContentsArray(response, uri)
	}
}

// ensureArray ensures that the provided value is an array
func ensureArray(value interface{}) []interface{} {
	// If already an array, return it
	if array, ok := value.([]interface{}); ok {
		return array
	}

	// If it's an array of maps, convert it
	if array, ok := value.([]map[string]interface{}); ok {
		result := make([]interface{}, len(array))
		for i, item := range array {
			result[i] = item
		}
		return result
	}

	// Otherwise, create a new array with the value
	return []interface{}{value}
}
