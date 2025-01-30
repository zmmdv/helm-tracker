package helm

import (
    "fmt"
    "os"
    "strings"
    "runtime"
    "time"
    "regexp"
    "strconv"
    "helm.sh/helm/v3/pkg/action"
    "helm.sh/helm/v3/pkg/cli"
    "helm.sh/helm/v3/pkg/repo"
    "helm.sh/helm/v3/pkg/release"
    "k8s.io/client-go/kubernetes"
    "github.com/sirupsen/logrus"
    "helm.sh/helm/v3/pkg/getter"
    "gopkg.in/yaml.v2"
    "github.com/Masterminds/semver/v3"
)

const (
    defaultCheckInterval = "6h"
)

type Schedule struct {
    interval time.Duration
    weekday  time.Weekday
    isWeekly bool
}

type ChartMapping struct {
    InstalledName string `yaml:"installed_name"`
    RemoteName    string `yaml:"remote_name"`
}

type RepoConfig struct {
    Name   string                 `yaml:"name"`
    URL    string                 `yaml:"url"`
    Charts map[string]ChartMapping `yaml:"charts"`
}

type NotificationConfig struct {
    Enabled bool `yaml:"enabled"`
}

type Config struct {
    Repositories  []RepoConfig      `yaml:"repositories"`
    Notifications NotificationConfig `yaml:"notifications"`
}

type Monitor struct {
    client       *kubernetes.Clientset
    log          *logrus.Logger
    config       *Config
    notifier     *NotificationService
}

func parseInterval(s string) (*Schedule, error) {
    // Handle weekly format first
    weeklyRegex := regexp.MustCompile(`^(\d+)w(?:/(\w+))?$`)
    if matches := weeklyRegex.FindStringSubmatch(s); matches != nil {
        weekday := time.Monday // Default to Monday if no day specified
        if matches[2] != "" {
            var err error
            weekday, err = parseWeekday(matches[2])
            if err != nil {
                return nil, fmt.Errorf("invalid weekday: %v", err)
            }
        }
        return &Schedule{
            interval: time.Hour * 24 * 7,
            weekday:  weekday,
            isWeekly: true,
        }, nil
    }

    // Handle regular intervals (minutes, hours, days)
    value, err := parseRegularInterval(s)
    if err != nil {
        return nil, fmt.Errorf("invalid interval format. Valid formats: '1m', '1h', '1d', '1w', or '1w/monday' for weekly schedule")
    }

    return &Schedule{interval: value, isWeekly: false}, nil
}

func parseWeekday(day string) (time.Weekday, error) {
    days := map[string]time.Weekday{
        "sunday":    time.Sunday,
        "monday":    time.Monday,
        "tuesday":   time.Tuesday,
        "wednesday": time.Wednesday,
        "thursday":  time.Thursday,
        "friday":    time.Friday,
        "saturday":  time.Saturday,
    }

    if weekday, ok := days[strings.ToLower(day)]; ok {
        return weekday, nil
    }
    return 0, fmt.Errorf("invalid weekday: %s", day)
}

func parseRegularInterval(s string) (time.Duration, error) {
    if strings.HasSuffix(s, "d") {
        days, err := strconv.Atoi(s[:len(s)-1])
        if err != nil {
            return 0, fmt.Errorf("invalid day value: %v", err)
        }
        return time.Hour * 24 * time.Duration(days), nil
    }

    // Handle minutes and hours directly with time.ParseDuration
    duration, err := time.ParseDuration(s)
    if err != nil {
        return 0, fmt.Errorf("invalid duration: %v", err)
    }

    return duration, nil
}

func NewMonitor(client *kubernetes.Clientset, log *logrus.Logger) *Monitor {
    logLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))
    switch logLevel {
    case "debug":
        log.SetLevel(logrus.DebugLevel)
    case "info":
        log.SetLevel(logrus.InfoLevel)
    case "warn", "warning":
        log.SetLevel(logrus.WarnLevel)
    case "error":
        log.SetLevel(logrus.ErrorLevel)
    default:
        log.SetLevel(logrus.InfoLevel)
    }

    m := &Monitor{
        client: client,
        log:    log,
    }
    
    config, err := m.loadConfig()
    if err != nil {
        log.Errorf("Failed to load config: %v", err)
        return m
    }
    m.config = config
    m.notifier = NewNotificationService(config.Notifications)
    
    return m
}

func (m *Monitor) loadConfig() (*Config, error) {
    m.log.Debug("Loading repository configuration")
    configPath := "/etc/helm-tracker/repositories.yaml"
    
    if _, err := os.Stat(configPath); os.IsNotExist(err) {
        return nil, fmt.Errorf("config file does not exist at %s", configPath)
    }
    
    data, err := os.ReadFile(configPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file: %v", err)
    }

    var config Config
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("failed to parse config: %v", err)
    }

    // Check for repository duplications
    repoNames := make(map[string][]string)
    for _, repo := range config.Repositories {
        repoNames[repo.Name] = append(repoNames[repo.Name], repo.URL)
    }

    var duplicateErrors []string
    for name, urls := range repoNames {
        if len(urls) > 1 {
            duplicateErrors = append(duplicateErrors, 
                fmt.Sprintf("Repository '%s' is duplicated with URLs: %s", 
                    name, strings.Join(urls, ", ")))
        }
    }

    // Check for chart duplications
    chartInstalls := make(map[string][]string)
    for _, repo := range config.Repositories {
        for _, chart := range repo.Charts {
            chartInstalls[chart.InstalledName] = append(chartInstalls[chart.InstalledName], 
                fmt.Sprintf("%s (%s)", repo.Name, repo.URL))
        }
    }

    for name, repos := range chartInstalls {
        if len(repos) > 1 {
            duplicateErrors = append(duplicateErrors, 
                fmt.Sprintf("Chart installation name '%s' is duplicated in repositories: %s", 
                    name, strings.Join(repos, ", ")))
        }
    }

    if len(duplicateErrors) > 0 {
        return nil, fmt.Errorf("Configuration error - found duplications:\n%s", 
            strings.Join(duplicateErrors, "\n"))
    }

    m.log.Debugf("Loaded configuration with %d repositories", len(config.Repositories))
    return &config, nil
}

func (m *Monitor) Start() {
    intervalStr := os.Getenv("CHECK_INTERVAL")
    if intervalStr == "" {
        intervalStr = defaultCheckInterval
        m.log.Infof("No CHECK_INTERVAL specified, using default: %s", defaultCheckInterval)
    }

    schedule, err := parseInterval(intervalStr)
    if err != nil {
        m.log.Errorf("Invalid check interval '%s': %v", intervalStr, err)
        m.log.Info("Using default interval: 6h")
        schedule, _ = parseInterval(defaultCheckInterval)
    }

    m.log.Infof("Starting helm-monitor with check interval: %s", intervalStr)
    
    for {
        m.CheckUpdates()
        
        // Calculate next check time
        nextCheckTime := time.Now().Add(schedule.interval)
        if schedule.isWeekly {
            nextCheckTime = m.nextWeekday(time.Now(), schedule.weekday)
        }
        
        m.log.Info("========================================")
        m.log.Info("Helm release check completed")
        m.log.Infof("Next check scheduled for: %s", nextCheckTime.Format("2006-01-02 15:04:05"))
        m.log.Info("========================================")

        // Sleep until next check
        if schedule.isWeekly {
            time.Sleep(time.Until(nextCheckTime))
        } else {
            time.Sleep(schedule.interval)
        }
    }
}

func (m *Monitor) nextWeekday(now time.Time, weekday time.Weekday) time.Time {
    daysUntil := int(weekday - now.Weekday())
    if daysUntil <= 0 {
        daysUntil += 7
    }
    nextRun := now.Add(time.Hour * 24 * time.Duration(daysUntil))
    return time.Date(nextRun.Year(), nextRun.Month(), nextRun.Day(), 0, 0, 0, 0, now.Location())
}

func (m *Monitor) findChartInfo(releaseName string) (string, string) {
    if m.config == nil {
        m.log.Error("Configuration not loaded")
        return "", ""
    }

    m.log.Debugf("Looking for repository for release: %s", releaseName)
    
    for _, repo := range m.config.Repositories {
        for _, chartMapping := range repo.Charts {
            if releaseName == chartMapping.InstalledName {
                m.log.Debugf("Found matching repository %s for release %s, remote chart name: %s", 
                    repo.URL, releaseName, chartMapping.RemoteName)
                return repo.URL, chartMapping.RemoteName
            }
        }
    }
    
    return "", ""
}

func (m *Monitor) getLatestVersion(repoURL, chartName string) (string, error) {
    m.log.Debugf("Getting latest version for chart %s from repository %s", chartName, repoURL)
    
    settings := cli.New()
    
    tempDir, err := os.MkdirTemp("", "helm-cache-*")
    if err != nil {
        return "", fmt.Errorf("failed to create temp directory: %v", err)
    }
    defer os.RemoveAll(tempDir)
    
    settings.RepositoryCache = tempDir
    
    chartRepo := repo.Entry{
        URL: repoURL,
        Name: "temp-repo",
    }

    chartRepository, err := repo.NewChartRepository(&chartRepo, getter.All(settings))
    if err != nil {
        return "", fmt.Errorf("failed to create chart repository: %v", err)
    }

    index, err := chartRepository.DownloadIndexFile()
    if err != nil {
        return "", fmt.Errorf("failed to download repository index: %v", err)
    }
    defer os.Remove(index)

    indexFile, err := repo.LoadIndexFile(index)
    if err != nil {
        return "", fmt.Errorf("failed to load index file: %v", err)
    }

    chartVersions, ok := indexFile.Entries[chartName]
    if !ok {
        return "", fmt.Errorf("chart %s not found in repository", chartName)
    }

    if len(chartVersions) == 0 {
        return "", fmt.Errorf("no versions found for chart %s", chartName)
    }

    latestVersion := chartVersions[0].Version
    return latestVersion, nil
}

func (m *Monitor) CheckUpdates() {
    m.log.Debug("Starting CheckUpdates")
    
    // Get the check interval from environment
    intervalStr := os.Getenv("CHECK_INTERVAL")
    if intervalStr == "" {
        intervalStr = defaultCheckInterval
        m.log.Infof("No CHECK_INTERVAL specified, using default: %s", defaultCheckInterval)
    }

    schedule, err := parseInterval(intervalStr)
    if err != nil {
        m.log.Errorf("Invalid check interval '%s': %v", intervalStr, err)
        m.log.Info("Using default interval: 6h")
        schedule, _ = parseInterval(defaultCheckInterval)
    }

    settings := cli.New()
    
    actionConfig := new(action.Configuration)
    if err := actionConfig.Init(settings.RESTClientGetter(), "", "", m.log.Printf); err != nil {
        m.log.Errorf("Failed to init action config: %v", err)
        return
    }

    configuredReleases := make(map[string]struct{})
    for _, repo := range m.config.Repositories {
        for _, chartMapping := range repo.Charts {
            configuredReleases[chartMapping.InstalledName] = struct{}{}
        }
    }

    batchSize := 5
    client := action.NewList(actionConfig)
    client.AllNamespaces = true
    
    releases, err := client.Run()
    if err != nil {
        m.log.Errorf("Failed to list releases: %v", err)
        return
    }

    var releaseQueue []*release.Release
    for _, release := range releases {
        if _, exists := configuredReleases[release.Name]; exists {
            releaseQueue = append(releaseQueue, release)
        }
    }
    releases = nil
    runtime.GC()

    var updates []string
    for i := 0; i < len(releaseQueue); i += batchSize {
        end := i + batchSize
        if end > len(releaseQueue) {
            end = len(releaseQueue)
        }
        
        batch := releaseQueue[i:end]
        for _, release := range batch {
            repository, remoteChartName := m.findChartInfo(release.Name)
            if repository == "" || remoteChartName == "" {
                continue
            }

            currentVersion := release.Chart.Metadata.Version
            latestVersion, err := m.getLatestVersion(repository, remoteChartName)
            if err != nil {
                m.log.Errorf("Failed to get latest version for %s: %v", remoteChartName, err)
                continue
            }

            current, err := semver.NewVersion(currentVersion)
            if err != nil {
                m.log.Errorf("Failed to parse current version %s: %v", currentVersion, err)
                continue
            }

            latest, err := semver.NewVersion(latestVersion)
            if err != nil {
                m.log.Errorf("Failed to parse latest version %s: %v", latestVersion, err)
                continue
            }

            if latest.GreaterThan(current) {
                updateMsg := fmt.Sprintf("•    *release*: %s\n      *namespace*: %s\n      *installed*: %s\n      *latest in remote repo*: %s\n",
                    release.Name, 
                    release.Namespace, 
                    currentVersion, 
                    latestVersion)
                updates = append(updates, updateMsg)
                
                m.log.Infof("Update available for helm release: %s in namespace: %s, current version: %s, latest version: %s",
                    release.Name, release.Namespace, currentVersion, latestVersion)
            } else {
                m.log.Infof("Helm release %s in namespace: %s is up to date version: %s",
                    release.Name, release.Namespace, currentVersion)
            }

            runtime.GC()
        }
    }

    // Send notifications if there are any updates
    if len(updates) > 0 {
        if err := m.notifier.SendSlackNotification(updates, schedule.interval); err != nil {
            if strings.HasPrefix(err.Error(), "NOTIFICATION_SKIPPED:") {
                m.log.Info(strings.TrimPrefix(err.Error(), "NOTIFICATION_SKIPPED: "))
            } else {
                m.log.Errorf("Failed to send Slack notification: %v", err)
            }
        }
    }
}