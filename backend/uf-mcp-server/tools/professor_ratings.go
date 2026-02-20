package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
)

// -----------------------------------------------------------------------------
// Tool schema
// -----------------------------------------------------------------------------

type ProfessorRating struct {
	ProfessorName string `json:"name" description:"Professor name to search for (first, last, or full name)" required:"true"`
}

func (t *ProfessorRating) Name() string { return "ProfessorRating" }

func (t *ProfessorRating) Description() string {
	return "Fetches a professor's Quality rating, Would Take Again percentage, Level of Difficulty, and latest comments from RateMyProfessors.com (school ID 1100)."
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------
/*
func normalizeName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parts := strings.Fields(raw)
	for i, p := range parts {
		p = strings.ToLower(p)
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}
*/
// -----------------------------------------------------------------------------
// GraphQL API client
// -----------------------------------------------------------------------------

const gqlURL = "https://www.ratemyprofessors.com/graphql"

type gqlRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type gqlResponse struct {
	Data struct {
		Teacher struct {
			FirstName             string  `json:"firstName"`
			LastName              string  `json:"lastName"`
			AvgDifficulty         float64 `json:"avgDifficulty"`
			WouldTakeAgainPercent int     `json:"wouldTakeAgainPercent"`
			AvgRating             float64 `json:"avgRating"`
			Ratings               []struct {
				Comment string `json:"comment"`
			} `json:"ratings"`
		} `json:"teacher"`
	} `json:"data"`
}

func fetchProfessorMetrics(profID string) (string, error) {
	query := `
    query TeacherRatings($id: ID!) {
        teacher(id: $id) {
            firstName
            lastName
            avgDifficulty
            wouldTakeAgainPercent
            avgRating
            ratings(first: 5) {
                comment
            }
        }
    }`

	reqBody := gqlRequest{
		Query: query,
		Variables: map[string]interface{}{
			"id": profID,
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	log.Printf("Calling GraphQL with professor ID: %s", profID)

	resp, err := http.Post(gqlURL, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("GraphQL request failed: %v", err)
	}
	defer resp.Body.Close()

	var gqlResp gqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return "", fmt.Errorf("failed to decode GraphQL response: %v", err)
	}

	t := gqlResp.Data.Teacher
	if t.FirstName == "" && t.LastName == "" {
		return "", fmt.Errorf("no teacher data returned")
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Professor: %s %s\n", t.FirstName, t.LastName))
	b.WriteString(fmt.Sprintf("Quality Rating: %.1f\n", t.AvgRating))
	b.WriteString(fmt.Sprintf("Would Take Again: %d%%\n", t.WouldTakeAgainPercent))
	b.WriteString(fmt.Sprintf("Level of Difficulty: %.1f\n\n", t.AvgDifficulty))

	b.WriteString("Latest Comments:\n")
	if len(t.Ratings) == 0 {
		b.WriteString("  No comments available.\n")
	} else {
		for i, c := range t.Ratings {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, c.Comment))
		}
	}
	return b.String(), nil
}

// -----------------------------------------------------------------------------
// HTML fallback scraper
// -----------------------------------------------------------------------------

func fetchProfessorDetailFallback(detailURL, profName, qualityHint string) (string, error) {
	resp, err := http.Get(detailURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch detail page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("detail page returned %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse detail HTML: %v", err)
	}

	// Scrape quality rating
	quality := strings.TrimSpace(doc.Find("div[class^='RatingValue__Numerator']").First().Text())
	if quality == "" {
		quality = strings.TrimSpace(doc.Find(".CardNumRating__CardNumRatingNumber-sc-17t4b9u-2").First().Text())
	}
	if quality == "" {
		quality = qualityHint
	}

	var wouldTakeAgain, levelDifficulty string
	doc.Find("div[class^='FeedbackItem__StyledFeedbackItem']").Each(func(i int, item *goquery.Selection) {
		num := strings.TrimSpace(item.Find("div[class^='FeedbackItem__FeedbackNumber']").First().Text())
		desc := strings.TrimSpace(item.Find("div[class^='FeedbackItem__FeedbackDescription']").First().Text())
		dl := strings.ToLower(desc)

		if strings.Contains(dl, "would take again") && wouldTakeAgain == "" {
			wouldTakeAgain = num
		} else if strings.Contains(dl, "level of difficulty") && levelDifficulty == "" {
			levelDifficulty = num
		}
	})

	if wouldTakeAgain == "" {
		wouldTakeAgain = "(not available)"
	}
	if levelDifficulty == "" {
		levelDifficulty = "(not available)"
	}

	var comments []string
	doc.Find(".Comments__StyledComments-dzzyvm-0, .Comments__CommentContainer-sc-1t3w3z-0").Each(func(i int, s *goquery.Selection) {
		if i < 10 {
			comment := strings.TrimSpace(s.Text())
			if comment != "" {
				comments = append(comments, comment)
			}
		}
	})

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Professor: %s\n", profName))
	b.WriteString(fmt.Sprintf("Quality Rating: %s\n", quality))
	b.WriteString(fmt.Sprintf("Would Take Again: %s\n", wouldTakeAgain))
	b.WriteString(fmt.Sprintf("Level of Difficulty: %s\n\n", levelDifficulty))

	b.WriteString("Latest Comments:\n")
	if len(comments) == 0 {
		b.WriteString("  No comments available.\n")
	} else {
		for i, c := range comments {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, c))
		}
	}
	return b.String(), nil
}

// -----------------------------------------------------------------------------
// Search scraper (to get professor ID)
// -----------------------------------------------------------------------------

func fetchProfessorID(userInput string) (string, string, string, error) {
	baseURL := "https://www.ratemyprofessors.com/search/professors/1100"
	encodedQuery := url.QueryEscape(strings.TrimSpace(userInput))
	fullURL := fmt.Sprintf("%s?q=%s", baseURL, encodedQuery)

	resp, err := http.Get(fullURL)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to fetch search page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("search page returned %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse search HTML: %v", err)
	}

	var profID, profName, detailURL string
	doc.Find("a").EachWithBreak(func(i int, s *goquery.Selection) bool {
		name := strings.TrimSpace(s.Find("[data-qa='teacher-name']").Text())
		if name == "" {
			name = strings.TrimSpace(s.Find(".CardName__StyledCardName-sc-1gyrgim-0").Text())
		}
		if name == "" {
			return true // continue
		}
		link, exists := s.Attr("href")
		if exists && strings.Contains(link, "/professor/") {
			parts := strings.Split(link, "/")
			profID = parts[len(parts)-1]
			profName = name
			detailURL = "https://www.ratemyprofessors.com" + link
			log.Printf("Selected professor ID: %s, Name: %s, DetailURL: %s", profID, profName, detailURL)
			return false // break once we find the intended professor
		}
		return true
	})

	if profID == "" {
		return "", "", "", fmt.Errorf("no professor ID found for '%s'", userInput)
	}
	return profID, profName, detailURL, nil
}

// -----------------------------------------------------------------------------
// MCP handler
// -----------------------------------------------------------------------------

func handleProfessorRating(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	var request ProfessorRating
	if err := protocol.VerifyAndUnmarshal(req.RawArguments, &request); err != nil {
		return nil, err
	}

	profID, profName, detailURL, err := fetchProfessorID(request.ProfessorName)
	if err != nil {
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{Type: "text", Text: fmt.Sprintf("Error fetching professor ID: %v", err)},
			},
			IsError: true,
		}, nil
	}

	// Try GraphQL first
	result, err := fetchProfessorMetrics(profID)
	if err != nil {
		log.Printf("GraphQL failed: %v. Falling back to HTML scraping.", err)
		// Fallback to HTML scraping
		result, err = fetchProfessorDetailFallback(detailURL, profName, "(quality not available)")
		if err != nil {
			return &protocol.CallToolResult{
				Content: []protocol.Content{
					&protocol.TextContent{Type: "text", Text: fmt.Sprintf("Error fetching metrics: %v", err)},
				},
				IsError: true,
			}, nil
		}
	}

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{Type: "text", Text: result},
		},
		IsError: false,
	}, nil
}

// -----------------------------------------------------------------------------
// Registration
// -----------------------------------------------------------------------------

func GetProfessorRatingTool() (*protocol.Tool, server.ToolHandlerFunc) {
	toolStruct := ProfessorRating{}
	tool, err := protocol.NewTool(
		toolStruct.Name(),
		toolStruct.Description(),
		toolStruct,
	)
	if err != nil {
		log.Fatalf("Failed to create ProfessorRating tool: %v", err)
	}
	return tool, handleProfessorRating
}
