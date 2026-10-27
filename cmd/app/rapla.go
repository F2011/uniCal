package app

import (
	"crypto/tls"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
)

type WebCal struct {
	cal *ics.Calendar
}

// Creating a new web calendar (ics) instance based on a provided URL
func FetchNewWebCalInstance(url string) (*WebCal, error) {
	// Check if running in CI environment
	isCI := os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true"

	// Create a custom HTTP client with proper timeout and certificate handling
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Only skip certificate verification in CI environments where CA certs might be outdated
	if isCI {
		tlsConfig.InsecureSkipVerify = true
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	cal, err := ics.ParseCalendar(resp.Body)
	if err != nil {
		return nil, err
	}
	return &WebCal{cal: cal}, nil
}

// Save the filtered calendar to a file
func (webCal *WebCal) SaveFilteredICal(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(webCal.cal.Serialize())
	if err != nil {
		return err
	}

	return nil
}

// Functions that operate on the calendar

// Filter events based on provided blocklist
func (webCal *WebCal) FilterEvents(blocklist []string, notes map[string]string) {
	numberFilter := regexp.MustCompile(`^\d{7} - `)
	// Create a new calendar and copy relevant properties from the original
	filteredCal := ics.NewCalendar()
	for _, event := range webCal.cal.Events() {
		summaryProperty := event.GetProperty(ics.ComponentPropertySummary)
		summaryProperty.Value = numberFilter.ReplaceAllString(summaryProperty.Value, "")
		uidProperty := event.GetProperty(ics.ComponentPropertyUniqueId)

		if summaryProperty != nil && !(slices.Contains(blocklist, summaryProperty.Value) || slices.Contains(blocklist, uidProperty.Value)) {
			event := webCal.addNotesToEvent(event, notes)

			filteredCal.AddVEvent(event)
		}
	}
	webCal.cal = filteredCal
}

func (webCal *WebCal) addNotesToEvent(event *ics.VEvent, notes map[string]string) *ics.VEvent {
	eventTitle := event.GetProperty(ics.ComponentPropertySummary).Value
	if eventTitle != "" {
		if note, exists := notes[strings.ToLower(eventTitle)]; exists {
			// Get existing description
			existingDescription := ""
			if descProp := event.GetProperty(ics.ComponentPropertyDescription); descProp != nil {
				existingDescription = descProp.Value
			}
			// Append the note to the existing description (using proper line breaks)
			newDescription := existingDescription
			if existingDescription != "" {
				newDescription += "  --- Notes ---  "
			} else {
				newDescription = "  --- Notes ---  "
			}
			newDescription += note
			// Update the event's description property
			event.RemoveProperty(ics.ComponentPropertyDescription)
			event.SetProperty(ics.ComponentPropertyDescription, newDescription)
			return event
		}
	}
	return event
}
