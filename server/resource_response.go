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
	case "draft":
		// For now, draft has the same format as 2025-03-26
		// In the future, if they diverge, implement a separate formatter
		return formatResourceV20250326(uri, result)
	default:
		// If version is unknown, use the most recent format
		return formatResourceV20250326(uri, result)
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

// formatResourceV20250326 formats a response for the 2025-03-26 and draft MCP specifications
func formatResourceV20250326(uri string, result interface{}) map[string]interface{} {
	// Special handling for string results
	if str, ok := result.(string); ok {
		return map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"uri":  uri,
					"text": str, // Required field at this level
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": str,
						},
					},
				},
			},
		}
	}

	// Handle map results, ensuring proper structure
	if resultMap, ok := result.(map[string]interface{}); ok {
		// Special case for empty content array
		if content, hasContent := resultMap["content"]; hasContent {
			if contentArr, isArray := content.([]interface{}); isArray && len(contentArr) == 0 {
				// This is an explicitly empty content array, preserve it
				return map[string]interface{}{
					"contents": []map[string]interface{}{
						{
							"uri":     uri,
							"text":    "Empty content", // Required field at contents level
							"content": []interface{}{}, // Keep the empty array
						},
					},
				}
			}
		}

		// Case 1: If the result already has a properly formatted "contents" array (2025-03-26 format)
		if contents, hasContents := resultMap["contents"]; hasContents {
			contentsArray := ensureArray(contents)

			// If contents is empty, create a default content item
			if len(contentsArray) == 0 {
				return map[string]interface{}{
					"contents": []map[string]interface{}{
						{
							"uri":  uri,
							"text": "Empty content",
							"content": []map[string]interface{}{
								{
									"type": "text",
									"text": "Empty content",
								},
							},
						},
					},
				}
			}

			// Validate each content item in the contents array
			validContents := make([]map[string]interface{}, 0, len(contentsArray))
			for _, item := range contentsArray {
				if contentItem, ok := item.(map[string]interface{}); ok {
					// Ensure URI is set
					if contentItem["uri"] == nil || contentItem["uri"] == "" {
						contentItem["uri"] = uri
					}

					// Ensure content array exists and is properly structured
					innerContent, hasInnerContent := contentItem["content"]
					if !hasInnerContent || innerContent == nil {
						// Create default content array if missing
						contentItem["content"] = []map[string]interface{}{
							{
								"type": "text",
								"text": "Default content",
							},
						}

						// Also ensure text field exists
						if contentItem["text"] == nil {
							contentItem["text"] = "Default content"
						}
					} else if innerArr, isArray := innerContent.([]interface{}); isArray && len(innerArr) == 0 {
						// This is an explicitly empty content array, preserve it
						contentItem["content"] = []interface{}{}
						// But ensure text field exists at content level (required for rendering)
						if contentItem["text"] == nil {
							contentItem["text"] = "Empty content"
						}
					} else {
						// Validate inner content array
						innerContentArray := ensureArray(innerContent)

						// Process and validate each inner content item
						validInnerContent := make([]map[string]interface{}, 0, len(innerContentArray))
						for _, innerItem := range innerContentArray {
							if innerItemMap, ok := innerItem.(map[string]interface{}); ok {
								// Ensure required type field
								if innerItemMap["type"] == nil {
									innerItemMap["type"] = "text"
								}

								// Handle specific content types
								switch innerItemMap["type"] {
								case "text":
									if innerItemMap["text"] == nil {
										innerItemMap["text"] = "Default text"
									}
								case "image":
									if innerItemMap["imageUrl"] == nil {
										// Convert to text if missing required fields
										innerItemMap["type"] = "text"
										innerItemMap["text"] = "Invalid image (missing URL)"
									} else if innerItemMap["altText"] == nil {
										innerItemMap["altText"] = "Image"
									}
								case "link":
									if innerItemMap["url"] == nil {
										// Convert to text if missing required fields
										innerItemMap["type"] = "text"
										innerItemMap["text"] = "Invalid link (missing URL)"
									} else if innerItemMap["title"] == nil {
										innerItemMap["title"] = "Link"
									}
								case "audio":
									// Handle audio content format for 2025-03-26 and draft differently
									// This is called from formatResourceV20250326 which handles both 2025-03-26 and draft
									// Let's check if audioUrl is present to detect draft spec format
									hasAudioUrl := innerItemMap["audioUrl"] != nil

									if hasAudioUrl {
										// Draft spec uses audioUrl
										if innerItemMap["mimeType"] == nil {
											innerItemMap["mimeType"] = "audio/mpeg" // Default mime type
										}
										// Remove data field if present to avoid confusion
										delete(innerItemMap, "data")
									} else {
										// 2025-03-26 spec uses data field
										if innerItemMap["data"] == nil {
											// No data and no audioUrl - invalid audio
											innerItemMap["type"] = "text"
											innerItemMap["text"] = "Invalid audio (missing data)"
										} else if innerItemMap["mimeType"] == nil {
											innerItemMap["mimeType"] = "audio/mpeg" // Default mime type
										}
									}
								}

								validInnerContent = append(validInnerContent, innerItemMap)
							} else {
								// Convert non-map items to text
								validInnerContent = append(validInnerContent, map[string]interface{}{
									"type": "text",
									"text": fmt.Sprintf("%v", innerItem),
								})
							}
						}

						// If no valid inner content items, keep an empty array
						if len(validInnerContent) == 0 {
							contentItem["content"] = []interface{}{}
						} else {
							// Update the content array with validated items
							contentItem["content"] = validInnerContent
						}

						// Ensure text field at content level exists (required for rendering)
						if contentItem["text"] == nil {
							// Use the text from the first text content item or a default
							foundText := false
							for _, innerItemMap := range validInnerContent {
								if innerItemMap["type"] == "text" && innerItemMap["text"] != nil {
									contentItem["text"] = innerItemMap["text"]
									foundText = true
									break
								}
							}

							if !foundText {
								contentItem["text"] = "Content"
							}
						}
					}

					validContents = append(validContents, contentItem)
				} else {
					// Convert non-map items to proper content structure
					validContents = append(validContents, map[string]interface{}{
						"uri":  uri,
						"text": fmt.Sprintf("%v", item),
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": fmt.Sprintf("%v", item),
							},
						},
					})
				}
			}

			// Update contents with validated items
			resultMap["contents"] = validContents
			return resultMap
		}

		// Case 2: Handle content array (2024-11-05 format) and convert to 2025-03-26 format
		if content, hasContent := resultMap["content"]; hasContent {
			contentArray := ensureArray(content)

			// Special case for empty content array
			if len(contentArray) == 0 {
				return map[string]interface{}{
					"contents": []map[string]interface{}{
						{
							"uri":     uri,
							"text":    "Empty content", // Required field at contents level
							"content": []interface{}{}, // Keep the empty array
						},
					},
				}
			}

			// Create a contents item with the content array nested inside
			contentsItem := map[string]interface{}{
				"uri":     uri,
				"content": contentArray,
			}

			// Determine the text field for the contents level
			if len(contentArray) > 0 {
				foundText := false
				for _, contentItem := range contentArray {
					if itemMap, ok := contentItem.(map[string]interface{}); ok {
						if itemMap["type"] == "text" && itemMap["text"] != nil {
							contentsItem["text"] = itemMap["text"]
							foundText = true
							break
						}
					}
				}

				if !foundText {
					contentsItem["text"] = "Content"
				}
			} else {
				contentsItem["text"] = "Empty content"
				// Add a default text content item
				contentsItem["content"] = []interface{}{}
			}

			// Create the 2025-03-26 structure
			// Keep other fields like metadata if they exist
			result := map[string]interface{}{
				"contents": []map[string]interface{}{contentsItem},
			}

			// Copy metadata if present
			if metadata, hasMetadata := resultMap["metadata"]; hasMetadata {
				result["metadata"] = metadata
			}

			return result
		}

		// Case 3: No recognized structure, create a default one
		defaultText := "Default content"
		if len(resultMap) > 0 {
			jsonBytes, err := json.Marshal(resultMap)
			if err == nil {
				defaultText = string(jsonBytes)
			}
		}

		return map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"uri":  uri,
					"text": defaultText,
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": defaultText,
						},
					},
				},
			},
		}
	}

	// For any other type, convert to JSON and format as text
	jsonData, err := json.Marshal(result)
	if err != nil {
		return formatResourceV20250326(uri, fmt.Sprintf("%v", result))
	}
	return formatResourceV20250326(uri, string(jsonData))
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
