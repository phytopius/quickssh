package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/BurntSushi/toml"

	tea "github.com/charmbracelet/bubbletea"
	ssh_config "github.com/kevinburke/ssh_config"
)

type Config struct {
	Hosts []SSHHost `toml:"hosts"`
}

type SSHHost struct {
	Host         string   `toml:"host"`
	HostName     string   `toml:"hostname"`
	User         string   `toml:"user"`
	ForwardAgent bool     `toml:"forward_agent"`
	Tags         []string `tomo:"ags"`
	Description  string   `toml:"description"`
}

type sshConfigEntry struct {
	Host         string
	HostName     string
	User         string
	ForwardAgent string
}

func loadConfig(path string) (*Config, error) {
	var config Config
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func saveConfig(path string, config *Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(config)
}

type model struct {
	mode        string
	entries     []SSHHost
	choices     []string
	cursor      int
	selected    map[int]struct{}
	inputBuffer []string
	inputField  int
	isAdding    bool
}

var fields = []string{"Host", "HostName", "User", "ForwardAgent", "Tags", "Description"}

func initialModel(config Config) model {
	var choices []string
	for _, entry := range config.Hosts {
		choices = append(choices, entry.Host)
	}
	return model{
		entries:     config.Hosts,
		choices:     choices,
		selected:    make(map[int]struct{}),
		inputBuffer: make([]string, len(fields)),
		isAdding:    false,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.isAdding {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEnter:
				m.inputField++
				if m.inputField >= len(fields) {
					// Done, construct new SSHHost
					newHost := SSHHost{
						Host:         m.inputBuffer[0],
						HostName:     m.inputBuffer[1],
						User:         m.inputBuffer[2],
						ForwardAgent: m.inputBuffer[3] == "true",
						Tags:         []string{}, // you could support this later
						Description:  m.inputBuffer[5],
					}
					m.entries = append(m.entries, newHost)
					m.choices = append(m.choices, newHost.Host)
					m.isAdding = false

					// Reload and Save
					dir, _ := os.Getwd()
					configPath := filepath.Join(dir, ".mysshconfig.toml")
					cfg := &Config{Hosts: m.entries}
					saveConfig(configPath, cfg)
				}
			case tea.KeyBackspace:
				if len(m.inputBuffer[m.inputField]) > 0 {
					m.inputBuffer[m.inputField] = m.inputBuffer[m.inputField][:len(m.inputBuffer[m.inputField])-1]
				}
			default:
				m.inputBuffer[m.inputField] += msg.String()
			}
		}
		return m, nil
	} else {
		switch msg := msg.(type) {
		// Is it a key press?
		case tea.KeyMsg:

			// Cool, what was the actual key pressed?
			switch msg.String() {

			// These keys should exit the program.
			case "ctrl+c", "q":
				return m, tea.Quit

			// The "up" and "k" keys move the cursor up
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}

			// The "down" and "j" keys move the cursor down
			case "down", "j":
				if m.cursor < len(m.choices)-1 {
					m.cursor++
				}

			case "a":
				m.isAdding = true
				m.inputBuffer = make([]string, len(fields))
				m.inputField = 0

			case "d":
				if len(m.entries) > 0 {
					index := m.cursor
					m.entries = append(m.entries[:index], m.entries[index+1:]...)
					m.choices = append(m.choices[:index], m.choices[index+1:]...)

					// Adjust cursor if necessary
					if m.cursor >= len(m.entries) && m.cursor > 0 {
						m.cursor--
					}

					// Save updated config to file
					dir, _ := os.Getwd()
					configPath := filepath.Join(dir, ".mysshconfig.toml")
					saveErr := saveConfig(configPath, &Config{Hosts: m.entries})
					if saveErr != nil {
						fmt.Fprintf(os.Stderr, "Error saving config: %v\n", saveErr)
					}
				}
			// The "enter" key and the spacebar (a literal space) toggle
			// the selected state for the item that the cursor is pointing at.
			case "enter", " ":
				var entry SSHHost
				if len(m.selected) > 0 {
					for i := range m.selected {
						entry = m.entries[i]
						break
					}
				} else {
					entry = m.entries[m.cursor]
				}

				connectArg := entry.User + "@" + entry.HostName
				fmt.Println("Connection args are: " + connectArg)
				cmd := exec.Command("ssh", connectArg)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr

				tea.Quit()
				err := cmd.Run()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error running ssh: %v\n", err)
				}

				os.Exit(0)
			}
		}

		// Return the updated model to the Bubble Tea runtime for processing.
		// Note that we're not returning a command.
		return m, nil
	}
}

func (m model) View() string {
	if m.isAdding {
		s := fmt.Sprintf("Adding new SSH host (%s):\n\n", fields[m.inputField])
		s += m.inputBuffer[m.inputField]
		s += "\n\nPress Enter to confirm field, Backspace to delete, q to quit."
		return s
	}

	// The header
	s := "What server do you want to connect to\n\n"

	// Iterate over our choices
	for i, choice := range m.choices {

		// Is the cursor pointing at this choice?
		cursor := " " // no cursor
		if m.cursor == i {
			cursor = ">" // cursor!
		}

		// Is this choice selected?
		checked := " " // not selected
		if _, ok := m.selected[i]; ok {
			checked = "x" // selected!
		}

		// Render the row
		s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, choice)
	}

	// The footer
	s += "\nPress q to quit.\n"

	// Send the UI for rendering
	return s
}

func ParseSSH() []sshConfigEntry {
	var entries []sshConfigEntry
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get home directory: %v\n", err)
		os.Exit(1)
	}
	file, err := os.Open(filepath.Join(homeDir, ".ssh", "config"))
	if err != nil {
		panic(err)
	}
	defer file.Close()

	cfg, err := ssh_config.Decode(file)
	if err != nil {
		panic(err)
	}

	seen := make(map[string]bool)
	for _, node := range cfg.Hosts {
		for _, pattern := range node.Patterns {
			if pattern.String() != "*" && !seen[pattern.String()] {
				seen[pattern.String()] = true
				entry := sshConfigEntry{
					Host:         pattern.String(),
					HostName:     ssh_config.Get(pattern.String(), "HostName"),
					User:         ssh_config.Get(pattern.String(), "User"),
					ForwardAgent: ssh_config.Get(pattern.String(), "ForwardAgent"),
				}

				entries = append(entries, entry)
			}
		}
	}
	return entries
}

func main() {
	dir, _ := os.Getwd()
	configPath := filepath.Join(dir, ".mysshconfig.toml")

	// Load Config
	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
	}

	p := tea.NewProgram(initialModel(*cfg))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
