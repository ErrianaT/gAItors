package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
)

// -----------------------------------------------------------------------------
// Tool schema
// -----------------------------------------------------------------------------

type UFClassSchedule struct {
	Term              string `json:"term" description:"Term code (e.g., 2261 for Fall 2025)" required:"true"`
	Category          string `json:"category" description:"Program type (e.g., RES or UGRD)" required:"true"`
	InstructorLast    string `json:"instructor_last" description:"Instructor's last name to filter (optional)" required:"false"`
	CourseCode        string `json:"course_code" description:"Course prefix and number (e.g., COP4020) (optional)" required:"false"`
	MaxSectionsOutput int    `json:"max_sections_output" description:"Limit number of sections returned (default 10, -1 for all)" required:"false"`
	Page              int    `json:"page" description:"Page number for pagination (0-based)" required:"false"`
}

func (t *UFClassSchedule) Name() string { return "UFClassSchedule" }

func (t *UFClassSchedule) Description() string {
	return "Fetches UF class schedule sections from one.ufl.edu API and supports pagination or 'show all'."
}

// -----------------------------------------------------------------------------
// Normalization helpers
// -----------------------------------------------------------------------------

func NormalizeTerm(term string) string {
	term = strings.TrimSpace(strings.ToLower(term))
	if strings.HasPrefix(term, "22") {
		return term
	}

	termMap := map[string]string{
		"fall 2025":   "2261",        	//2025-2026 academic year
		"spring 2026": "2261",			//2025-2026 academic year
		"summer 2026": "2261",			//2025-2026 academic year
		"fall 2026":   "2271",			//2026-2027 academic year
		"spring 2027": "2272",			//2026-2027 academic year
		"summer 2027": "2273",			//2026-2027 academic year
	}
	if val, ok := termMap[term]; ok {
		return val
	}
	return term
}

func NormalizeCategory(category string) string {
	category = strings.TrimSpace(strings.ToLower(category))
	categoryMap := map[string]string{
		"residential":   "RES",
		"res":           "RES",
		"undergraduate": "UGRD",
		"ugrd":          "UGRD",
		"graduate":      "GRAD",
		"grad":          "GRAD",
	}
	if val, ok := categoryMap[category]; ok {
		return val
	}
	return strings.ToUpper(category)
}

// -----------------------------------------------------------------------------
// API models
// -----------------------------------------------------------------------------

type CourseWrapper struct {
	Courses []Course `json:"COURSES"`
}

type Course struct {
	Code     string    `json:"code"`
	CourseID string    `json:"courseId"`
	Name     string    `json:"name"`
	Sections []Section `json:"sections"`
}

type Section struct {
	Number      string       `json:"number"`
	ClassNumber int          `json:"classNumber"`
	Display     string       `json:"display"`
	Credits     interface{}  `json:"credits"`
	Instructors []Instructor `json:"instructors"`
	MeetTimes   []MeetTime   `json:"meetTimes"`
}

type Instructor struct {
	Name string `json:"name"`
}

type MeetTime struct {
	MeetDays      []string `json:"meetDays"`
	MeetTimeBegin string   `json:"meetTimeBegin"`
	MeetTimeEnd   string   `json:"meetTimeEnd"`
	MeetBuilding  string   `json:"meetBuilding"`
	MeetRoom      string   `json:"meetRoom"`
}

// -----------------------------------------------------------------------------
// Client
// -----------------------------------------------------------------------------

const ufBase = "https://one.ufl.edu/apix/soc/schedule/"

var httpClient = &http.Client{Timeout: 12 * time.Second}

func fetchUFSections(term, category, instructorLast, courseCode string) ([]Section, error) {
	u, err := url.Parse(ufBase)
	if err != nil {
		return nil, fmt.Errorf("bad base url: %v", err)
	}
	q := u.Query()
	q.Set("term", term)
	q.Set("category", category)

	if strings.TrimSpace(instructorLast) != "" {
		q.Set("instructor", strings.TrimSpace(instructorLast))
	}
	if strings.TrimSpace(courseCode) != "" {
		q.Set("course-code", strings.TrimSpace(courseCode))
	}
	u.RawQuery = q.Encode()

	log.Printf("UFClassSchedule request URL: %s", u.String())

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("request build failed: %v", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	//log.Printf("Raw JSON: %s", string(bodyBytes))

	var wrappers []CourseWrapper
	if err := json.Unmarshal(bodyBytes, &wrappers); err != nil {
		return nil, fmt.Errorf("JSON decode failed: %v", err)
	}

	var sections []Section
	for _, cw := range wrappers {
		for _, c := range cw.Courses {
			for _, s := range c.Sections {
				sections = append(sections, s)
			}
		}
	}
	return sections, nil
}

// -----------------------------------------------------------------------------
// Formatting with pagination and show-all
// -----------------------------------------------------------------------------

func formatSections(sections []Section, maxOut, page int) string {
	if len(sections) == 0 {
		return "No sections found for the given filters."
	}

	if maxOut == -1 {
		return formatAllSections(sections)
	}

	if maxOut <= 0 {
		maxOut = 10
	}

	start := page * maxOut
	if start >= len(sections) {
		return fmt.Sprintf("No more sections available. Total sections: %d", len(sections))
	}
	end := start + maxOut
	if end > len(sections) {
		end = len(sections)
	}
	sections = sections[start:end]

	return formatAllSections(sections)
}

func formatAllSections(sections []Section) string {
	var b strings.Builder
	b.WriteString("Class sections:\n")
	for _, s := range sections {
		instr := "(no instructor listed)"
		if len(s.Instructors) > 0 {
			names := []string{}
			for _, i := range s.Instructors {
				names = append(names, i.Name)
			}
			instr = strings.Join(names, ", ")
		}

		b.WriteString(fmt.Sprintf("- %s (%s) — Instructor(s): %s\n", s.Display, s.Number, instr))
		if len(s.MeetTimes) == 0 {
			b.WriteString("  MeetTimes: (none)\n")
			continue
		}
		for _, m := range s.MeetTimes {
			days := strings.Join(m.MeetDays, "")
			loc := "(location n/a)"
			if m.MeetBuilding != "" || m.MeetRoom != "" {
				loc = fmt.Sprintf("%s %s", m.MeetBuilding, m.MeetRoom)
			}
			b.WriteString(fmt.Sprintf("  %s | %s–%s | %s\n", days, m.MeetTimeBegin, m.MeetTimeEnd, loc))
		}
	}
	return b.String()
}

// -----------------------------------------------------------------------------
// MCP handler
// -----------------------------------------------------------------------------

func handleUFClassSchedule(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	var request UFClassSchedule
	if err := protocol.VerifyAndUnmarshal(req.RawArguments, &request); err != nil {
		return nil, err
	}

	term := NormalizeTerm(request.Term)
	category := NormalizeCategory(request.Category)

	if term == "" || category == "" {
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{Type: "text", Text: "Error: 'term' and 'category' are required."},
			},
			IsError: true,
		}, nil
	}

	sections, err := fetchUFSections(term, category, request.InstructorLast, request.CourseCode)
	if err != nil {
		return &protocol.CallToolResult{
			Content: []protocol.Content{
				&protocol.TextContent{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	out := formatSections(sections, request.MaxSectionsOutput, request.Page)
	return &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{Type: "text", Text: out},
		},
		IsError: false,
	}, nil
}

// -----------------------------------------------------------------------------
// Registration
// -----------------------------------------------------------------------------

func GetUFClassScheduleTool() (*protocol.Tool, server.ToolHandlerFunc) {
	toolStruct := UFClassSchedule{}
	tool, err := protocol.NewTool(
		toolStruct.Name(),
		toolStruct.Description(),
		toolStruct,
	)
	if err != nil {
		log.Fatalf("Failed to create UFClassSchedule tool: %v", err)
	}
	return tool, handleUFClassSchedule
}
