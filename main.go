package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/build-on-aws/langchaingo-amazon-bedrock-llm/claude"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mitchellh/go-wordwrap"
	"github.com/tmc/langchaingo/llms"
	"golang.org/x/term"
)

// Main function is the entry point of the program
func main() {
	width, height, _ := term.GetSize(int(os.Stdout.Fd()))

	// Create a new Bubble Tea program instance, passing in our initial model
	p := tea.NewProgram(initialModel(width, height))

	// Run the Bubble Tea program loop
	if _, err := p.Run(); err != nil {

		// If there is an error, log it and exit
		log.Fatal(err)
	}
}

// Define errMsg as an alias for the error type
type (
	errMsg error
)

// Model stores the application state
type model struct {
	sidebarVP     viewport.Model // For sidebar content
	sidebar       []string       // Slice of strings to store chats
	messagesVP    viewport.Model // For displaying chat messages
	messages      []string       // Slice of strings to store chat messages
	input         textarea.Model // Textarea component for user input
	senderStyle   lipgloss.Style // Style for rendering user messages
	err           error          // Error field to store any errors
	claudeLLM     *claude.LLM    // Claude LLM instance
	claudeInitErr error          // Error during Claude LLM initialization
}

// Returns the initial model state
func initialModel(width, height int) model {
	// Calculate dynamic viewport heights and widths
	sidebarVPWidth := 25             // Adjust sidebar width as needed
	sidebarVPHeight := height - 2    // Adjust sidebar height as needed
	messagesVPWidth := width - 25    // Adjust sidebar width as needed
	messagesVPHeight := height - 7   // Adjust messages viewport height as needed
	inputTextareaWidth := width - 25 // Adjust input textarea height as needed
	inputTextareaHeight := 5         // Adjust input textarea height as needed

	// Create a new textarea component
	input := textarea.New()
	input.Placeholder = "Send a message..."
	input.Prompt = "┃ "
	input.CharLimit = 2048
	input.FocusedStyle.CursorLine = lipgloss.NewStyle()
	input.ShowLineNumbers = false
	input.Focus()
	input.SetWidth(inputTextareaWidth)
	input.SetHeight(inputTextareaHeight)
	input.KeyMap.InsertNewline.SetEnabled(true)

	// Create viewports
	sidebarVP := viewport.New(sidebarVPWidth, sidebarVPHeight)
	messagesVP := viewport.New(messagesVPWidth, messagesVPHeight)

	// Initialize Claude LLM instance
	claudeLLM, initialized := initializeClaudeLLM()
	if !initialized {
		return model{claudeInitErr: fmt.Errorf("failed to initialize Claude LLM")}
	}

	// Welcome message and sidebar content
	messagesVPWelcome := "Welcome to the chat room!\nType a message and press Enter to send."
	claudeEnterMessage := "Claude has entered the chat"
	botStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Italic(true)
	claudeEnterMsg := botStyle.Render(claudeEnterMessage)

	placeholderContent := []string{"+ New Chat Button\nChats\n  Chat1 (Edit/Delete)\n  Chat2 (Edit/Delete)\n  Chat3 (Edit/Delete)\n\n\n\nSome texts...\nSome texts..."}
	welcomeMessage := []string{messagesVPWelcome, claudeEnterMsg}

	sidebarVP.SetContent(strings.Join(placeholderContent, "\n"))
	messagesVP.SetContent(strings.Join(welcomeMessage, "\n"))

	// Return model with initial state
	return model{
		sidebarVP:   sidebarVP,
		sidebar:     []string{},
		messagesVP:  messagesVP,
		messages:    []string{},
		input:       input,
		senderStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
		err:         nil,
		claudeLLM:   claudeLLM,
	}
}

// Init returns the initial command to execute
func (m model) Init() tea.Cmd {
	return textarea.Blink // Return the textarea Blink command
}

// Handle TUI update loop
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Local variables for commands
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)
	width, _, _ := term.GetSize(int(os.Stdout.Fd()))

	m.input, tiCmd = m.input.Update(msg)           // Update textarea component
	m.messagesVP, vpCmd = m.messagesVP.Update(msg) // Update messages viewport

	// Check type of message
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc: // On ctrl+c or esc keypress
			fmt.Println(m.input.Value()) // Print textarea value
			return m, tea.Quit           // Return model and quit command
		case tea.KeyEnter: // On enter keypress
			userInput := m.input.Value()

			userStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("2")).
				Italic(true).
				Align(lipgloss.Left) // User message style

			botStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("5")).
				Align(lipgloss.Left) // Bot message style

			wrappedUserMsg := wordwrap.WrapString(userInput, uint(width-25))
			userMsg := userStyle.Render("You: " + wrappedUserMsg)

			m.messages = append(m.messages, userMsg) // Append user message

			// Update the viewport with the user's message immediately
			m.input.Reset() // Clear textarea
			m.messagesVP.SetContent(strings.Join(m.messages, "\n"))
			m.messagesVP.GotoBottom()

			// Call Claude LLM with the user input
			botMsg := ""
			claudeResponse, err := callClaudeLLM(userInput, m.claudeLLM)
			if err != nil {
				botMsg = botStyle.Render("Claude:" + "Error processing your request.")

			} else {
				// Wrap the response here before rendering
				wrappedBotResponse := wordwrap.WrapString(claudeResponse, uint(width-25))
				botMsg = botStyle.Render("Claude:" + wrappedBotResponse)
			}
			m.messages = append(m.messages, botMsg)

			m.messagesVP.SetContent(strings.Join(m.messages, "\n")) // Set viewport content
			m.messagesVP.GotoBottom()                               // Scroll viewport bottom
		}

	// Handle error messages
	case errMsg:
		m.err = msg
		return m, nil
	}

	// Return model and commands batch
	return m, tea.Batch(tiCmd, vpCmd)
}

// InitializeClaudeLLM initializes the Claude LLM instance and returns whether initialization was successful
func initializeClaudeLLM() (*claude.LLM, bool) {
	llm, err := claude.New("us-east-1")
	if err != nil {
		return nil, false
	}

	return llm, true
}

// Function to call Claude LLM
func callClaudeLLM(input string, llm *claude.LLM) (string, error) {
	// Call Claude LLM with the user input
	response, err := llm.Call(context.Background(), input, llms.WithMaxTokens(2048), llms.WithTemperature(0.5), llms.WithTopK(250))
	if err != nil {
		return "", err
	}

	return response, nil
}

// Render the UI view
func (m model) View() string {
	width, height, _ := term.GetSize(int(os.Stdout.Fd()))
	w := lipgloss.Width
	// Calculate viewport views
	sidebarView := m.sidebarVP.View()
	messagesView := m.messagesVP.View()
	inputTextareaView := m.input.View()

	// HeaderBar Style
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(lipgloss.Color("#FF5F87")).
		Align(lipgloss.Left).
		Padding(0, 1).
		Height(1).
		Width(width)

	// Sidebar on the left
	sideBarStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFF")).
		Background(lipgloss.Color("#333")).
		Align(lipgloss.Left).
		Padding(0, 1).
		Width(25).
		Height(height - 2)

	// Message view on the right of the sidebar
	messageViewStyle := lipgloss.NewStyle().
		Align(lipgloss.Left).
		Padding(0, 1).
		Width(width - 25).
		Height(height - 7)

	// Textarea view below the Message view and on the right of the sidebar
	textareaStyle := lipgloss.NewStyle().
		Align(lipgloss.Left).
		Width(width - 25).
		Height(5)

	// Status bar at the bottom
	statusBarNuggetStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Padding(0, 1)

	statusBarStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#343433", Dark: "#C1C6B2"}).
		Background(lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#353533"})

	statusKeyStyle := lipgloss.NewStyle().
		Inherit(statusBarStyle).
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(lipgloss.Color("#FF5F87")).
		Padding(0, 1).
		MarginRight(1)

	statusText := lipgloss.NewStyle().Inherit(statusBarStyle)

	statusBarEncodingStyle := statusBarNuggetStyle.Copy().
		Background(lipgloss.Color("#A550DF")).
		Align(lipgloss.Right)

	fishCakeStyle := statusBarNuggetStyle.Copy().Background(lipgloss.Color("#6124DF"))

	statusKey := statusKeyStyle.Render("STATUS")
	statusBarEncoding := statusBarEncodingStyle.Render("UTF-8")
	fishCake := fishCakeStyle.Render("🍥 Fish Cake")

	statusBarVal := statusText.Copy().
		Width(width - w(statusKey) - w(statusBarEncoding) - w(fishCake)).
		Render("Status Message")

	// Apply styles to the viewport content
	headerBar := headerStyle.Render("Kakapo 🦜")
	sidebar := sideBarStyle.Render(sidebarView)
	messagesViewArea := messageViewStyle.Render(messagesView)
	textArea := textareaStyle.Render(inputTextareaView)

	// Build the layout
	combinedStatusBar := lipgloss.JoinHorizontal(lipgloss.Bottom,
		statusKey,
		statusBarVal,
		statusBarEncoding,
		fishCake,
	)

	combinedChatView := lipgloss.JoinVertical(lipgloss.Top,
		messagesViewArea,
		textArea,
	)

	combinedMainView := lipgloss.JoinHorizontal(lipgloss.Bottom,
		sidebar,
		combinedChatView,
	)

	// Adjust the arrangement of views
	layout := lipgloss.JoinVertical(lipgloss.Top,
		headerBar,
		combinedMainView,
		combinedStatusBar,
	)

	return layout
}
