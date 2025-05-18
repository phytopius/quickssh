package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
				Render
)

func (i SSHHost) Title() string { return i.Host }
func (i SSHHost) Description() string {
	nicedescription := i.Desc + " " + strings.Join(i.Tags, "<")
	return nicedescription
}
func (i SSHHost) FilterValue() string { return i.Host }

// keys
type listKeyMap struct {
	insertItem key.Binding
	deleteItem key.Binding
	saveConfig key.Binding
}

// information for new keys
func newListKeyMap() *listKeyMap {
	return &listKeyMap{
		insertItem: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add item"),
		),
		deleteItem: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete item"),
		),
		saveConfig: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "save config"),
		),
	}
}

// content of the entire model
type model struct {
	list  list.Model
	keys  *listKeyMap
	hosts []SSHHost
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:

		if m.list.FilterState() == list.Filtering {
			break
		}
		switch {

		case key.Matches(msg, m.keys.insertItem):
			newHost := generateRandomHost()
			m.hosts = append(m.hosts, newHost)
			insCmd := m.list.InsertItem(0, newHost)
			statusCmd := m.list.NewStatusMessage(statusMessageStyle("Added " + newHost.HostName))
			return m, tea.Batch(insCmd, statusCmd)

		case key.Matches(msg, m.keys.deleteItem):
			currentItem := m.list.SelectedItem().(SSHHost)
			// remove from item list
			m.list.RemoveItem(m.list.Index())
			// remove from hsots list for config save
			newHosts := make([]SSHHost, 0, len(m.hosts))
			for _, p := range m.hosts {
				if p.Host != currentItem.Host {
					newHosts = append(newHosts, p)
				}
			}
			m.hosts = newHosts
			return m, tea.Batch()

		case key.Matches(msg, m.keys.saveConfig):
			config := &Config{Hosts: m.hosts}
			saveConfig(config)
			statusCmd := m.list.NewStatusMessage("Saved Config")
			return m, tea.Batch(statusCmd)
		}

	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	return appStyle.Render(m.list.View())
}

type Config struct {
	Hosts []SSHHost `toml:"hosts"`
}

func loadConfig(path string) (*Config, error) {
	var config Config
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func saveConfig(config *Config) error {
	dir, _ := os.Getwd()
	path := filepath.Join(dir, ".mysshconfig.toml")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(config)
}

type SSHHost struct {
	Host         string   `toml:"host"`
	HostName     string   `toml:"hostname"`
	User         string   `toml:"user"`
	ForwardAgent bool     `toml:"forward_agent"`
	Tags         []string `toml:"tags"`
	Desc         string   `toml:"description"`
}

func toItems(hosts []SSHHost) []list.Item {
	var items []list.Item
	for _, h := range hosts {
		items = append(items, h)
	}
	return items
}

func newModel() model {
	listKeys := newListKeyMap()
	dir, _ := os.Getwd()
	configPath := filepath.Join(dir, ".mysshconfig.toml")

	// Load Config
	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
	}

	items := toItems(cfg.Hosts)
	hosts := list.New(items, list.NewDefaultDelegate(), 0, 0)
	hosts.Title = "Available Hosts"
	hosts.Styles.Title = titleStyle
	hosts.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			listKeys.deleteItem,
			listKeys.insertItem,
			listKeys.saveConfig,
		}
	}
	return model{
		list:  hosts,
		keys:  listKeys,
		hosts: cfg.Hosts,
	}
}

func generateRandomHost() SSHHost {
	newHost := SSHHost{
		Host:         string(rand.Intn(100)),
		HostName:     string(rand.Intn(100)),
		User:         string(rand.Intn(100)),
		ForwardAgent: true,
		Tags:         []string{},
		Desc:         string(rand.Intn(100)),
	}

	return newHost
}

func main() {
	p := tea.NewProgram(newModel(), tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
