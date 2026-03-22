package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
)

type GymCamTool struct {
	Camera string `json:"camera,omitempty"`
}

func (GymCamTool) Name() string { return "GymCam" }
func (GymCamTool) Description() string {
	return "Return live camera images from UF RecSports (SRFC and SWRC)."
}

var GymCamMap = map[string]string{
	"srfc_weight":   "http://recsports.ufl.edu/cam/cam8.jpg",
	"srfc_cardio":   "http://recsports.ufl.edu/cam/cam7.jpg",
	"swrc_weight1":  "http://recsports.ufl.edu/cam/cam1.jpg",
	"swrc_weight2":  "http://recsports.ufl.edu/cam/cam4.jpg",
	"swrc_cardio":   "http://recsports.ufl.edu/cam/cam5.jpg",
	"swrc_basket12": "http://recsports.ufl.edu/cam/cam3.jpg",
	"swrc_basket34": "http://recsports.ufl.edu/cam/cam2.jpg",
	"swrc_basket56": "http://recsports.ufl.edu/cam/cam6.jpg",
}

var GymCamSynonyms = map[string]string{
	// SRFC synonyms
	"srfc":             "srfc_weight",
	"srfc gym":         "srfc_weight",
	"srfc weights":     "srfc_weight",
	"srfc weight room": "srfc_weight",
	"srfc cardio":      "srfc_cardio",
	"srfc treadmills":  "srfc_cardio",
	"srfc cardio room": "srfc_cardio",

	// SWRC weights
	"southwest rec":    "swrc_weight1",
	"swrc":             "swrc_weight1",
	"swrc weights":     "swrc_weight1",
	"swrc weight room": "swrc_weight1",
	"swrc weight1":     "swrc_weight1",
	"swrc weight2":     "swrc_weight2",

	// SWRC cardio
	"southwest rec cardio": "swrc_cardio",
	"swrc cardio":          "swrc_cardio",
	"swrc treadmills":      "swrc_cardio",
	"swrc cardio room":     "swrc_cardio",

	// SWRC basketball courts
	"swrc basketball court 1–2": "swrc_basket12",
	"swrc basketball 1-2":       "swrc_basket12",
	"swrc basket12":             "swrc_basket12",

	"swrc basketball court 3–4": "swrc_basket34",
	"swrc basketball 3-4":       "swrc_basket34",
	"swrc basket34":             "swrc_basket34",

	"swrc basketball court 5–6": "swrc_basket56",
	"swrc basketball 5-6":       "swrc_basket56",
	"swrc basket56":             "swrc_basket56",
}

func fetchImage(url string) ([]byte, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/jpeg"
	}
	// ✅ Normalize to just "image/jpeg" or "image/png"
	if strings.Contains(mimeType, "jpeg") {
		mimeType = "image/jpeg"
	} else if strings.Contains(mimeType, "png") {
		mimeType = "image/png"
	}

	return data, mimeType, nil
}

func handleGymCam(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	var args GymCamTool
	if err := protocol.VerifyAndUnmarshal(req.RawArguments, &args); err != nil {
		return nil, err
	}

	if args.Camera == "" {
		return &protocol.CallToolResult{
			IsError: true,
			Content: []protocol.Content{
				&protocol.TextContent{Type: "text", Text: "Error: camera argument missing."},
			},
		}, nil
	}

	normalized := strings.ToLower(args.Camera)
	if val, ok := GymCamSynonyms[normalized]; ok {
		args.Camera = val
	}

	url, ok := GymCamMap[args.Camera]
	if !ok {
		return &protocol.CallToolResult{
			IsError: true,
			Content: []protocol.Content{
				&protocol.TextContent{Type: "text", Text: fmt.Sprintf("Unknown camera ID: %s", args.Camera)},
			},
		}, nil
	}

	imageBytes, mimeType, err := fetchImage(url)
	if err != nil || len(imageBytes) == 0 {
		return &protocol.CallToolResult{
			IsError: true,
			Content: []protocol.Content{
				&protocol.TextContent{Type: "text", Text: fmt.Sprintf("Error fetching image for %s: %v", args.Camera, err)},
			},
		}, nil
	}

	encoded := base64.StdEncoding.EncodeToString(imageBytes)

	return &protocol.CallToolResult{
		IsError: false,
		Content: []protocol.Content{
			&protocol.TextContent{
				Type: "image",
				Text: fmt.Sprintf("%s|%s", mimeType, encoded),
			},
			&protocol.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Live feed for %s camera", args.Camera),
			},
		},
	}, nil
}

func GetGymCamTool() (*protocol.Tool, server.ToolHandlerFunc) {
	toolStruct := GymCamTool{}
	tool, err := protocol.NewTool(toolStruct.Name(), toolStruct.Description(), toolStruct)
	if err != nil {
		log.Fatalf("Failed to create GymCam tool: %v", err)
	}
	return tool, handleGymCam
}
