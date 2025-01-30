package helm

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "strings"
    "time"
    "strconv"
)

type SlackMessage struct {
    Text      string `json:"text"`
    Channel   string `json:"channel"`
    Timestamp string `json:"ts,omitempty"`
}

type SlackHistoryResponse struct {
    Ok       bool           `json:"ok"`
    Messages []SlackMessage `json:"messages"`
    Error    string        `json:"error,omitempty"`
}

type NotificationService struct {
    enabled     bool
    channelID   string
    botToken    string
}

func NewNotificationService(config NotificationConfig) *NotificationService {
    return &NotificationService{
        enabled:   config.Enabled,
        channelID: os.Getenv("SLACK_CHANNEL_ID"),
        botToken:  os.Getenv("SLACK_BOT_TOKEN"),
    }
}

func (n *NotificationService) getLastNotificationTime() (time.Time, error) {
    if n.channelID == "" || n.botToken == "" {
        return time.Time{}, fmt.Errorf("SLACK_CHANNEL_ID and SLACK_BOT_TOKEN are required")
    }

    // Only get the last message
    url := fmt.Sprintf("https://slack.com/api/conversations.history?channel=%s&limit=1", n.channelID)
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return time.Time{}, fmt.Errorf("failed to create request: %v", err)
    }

    req.Header.Set("Authorization", "Bearer "+n.botToken)
    
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return time.Time{}, fmt.Errorf("failed to get channel history: %v", err)
    }
    defer resp.Body.Close()

    var history SlackHistoryResponse
    if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
        return time.Time{}, fmt.Errorf("failed to decode response: %v", err)
    }

    if !history.Ok {
        return time.Time{}, fmt.Errorf("slack API error: %s", history.Error)
    }

    if len(history.Messages) == 0 {
        return time.Time{}, nil // No messages in channel
    }

    // Get the last message
    lastMsg := history.Messages[0]
    if strings.Contains(lastMsg.Text, "[HELM-MONITOR]") {
        ts := strings.Split(lastMsg.Timestamp, ".")[0]
        unix, err := strconv.ParseInt(ts, 10, 64)
        if err != nil {
            return time.Time{}, fmt.Errorf("failed to parse timestamp: %v", err)
        }
        return time.Unix(unix, 0), nil
    }

    return time.Time{}, nil // Last message wasn't from our monitor
}

func (n *NotificationService) shouldSendNotification(interval time.Duration) (bool, error) {
    lastNotification, err := n.getLastNotificationTime()
    if err != nil {
        return false, fmt.Errorf("failed to get last notification time: %v", err)
    }

    if lastNotification.IsZero() {
        return true, nil // No previous notification found, should send
    }

    nextAllowedTime := lastNotification.Add(interval)
    return time.Now().After(nextAllowedTime), nil
}

func (n *NotificationService) SendSlackNotification(updates []string, interval time.Duration) error {
    if !n.enabled {
        return nil // Notifications are disabled
    }

    if n.channelID == "" || n.botToken == "" {
        return fmt.Errorf("SLACK_CHANNEL_ID and SLACK_BOT_TOKEN are required")
    }

    if len(updates) == 0 {
        return nil // No updates to send
    }

    shouldSend, err := n.shouldSendNotification(interval)
    if err != nil {
        return fmt.Errorf("failed to check notification timing: %v", err)
    }

    if !shouldSend {
        return fmt.Errorf("NOTIFICATION_SKIPPED: Interval not passed yet")
    }

    // Create a formatted message with identifier
    message := "[HELM-MONITOR] *Helm Chart Updates Available:*\n"
    message += strings.Join(updates, "\n")
    message += fmt.Sprintf("\n\n_Next notification will be sent after: %s_", 
        time.Now().Add(interval).Format("2006-01-02 15:04:05"))

    payload := SlackMessage{
        Text:    message,
        Channel: n.channelID,
    }

    jsonPayload, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("failed to marshal JSON payload: %v", err)
    }

    req, err := http.NewRequest("POST", "https://slack.com/api/chat.postMessage", bytes.NewBuffer(jsonPayload))
    if err != nil {
        return fmt.Errorf("failed to create request: %v", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+n.botToken)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("failed to send Slack notification: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("failed to send Slack notification: received status code %d", resp.StatusCode)
    }

    return nil
}