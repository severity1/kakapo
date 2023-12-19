package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/build-on-aws/langchaingo-amazon-bedrock-llm/claude"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nathan-fiscaletti/consolesize-go"
	"github.com/tmc/langchaingo/llms"
)

// Main function is the entry point of the program
func main() {
	cols, rows := consolesize.GetConsoleSize()

	// Create a new Bubble Tea program instance, passing in our initial model
	p := tea.NewProgram(initialModel(cols, rows))

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
	messagesVP    viewport.Model // For displaying chat messages
	messages      []string       // Slice of strings to store chat messages
	input         textarea.Model // Textarea component for user input
	senderStyle   lipgloss.Style // Style for rendering user messages
	err           error          // Error field to store any errors
	claudeLLM     *claude.LLM    // Claude LLM instance
	claudeInitErr error          // Error during Claude LLM initialization
}

// Returns the initial model state
func initialModel(cols, rows int) model {
	// Calculate dynamic viewport heights and widths
	sidebarVPWidth := cols / 2   // Adjust sidebar width as needed
	sidebarVPHeight := rows      // Adjust sidebar height as needed
	messagesVPHeight := rows - 7 // Adjust messages viewport height as needed
	inputTextareaHeight := 5     // Adjust input textarea height as needed
	remainingCols := cols - sidebarVPWidth

	// Create a new textarea component
	input := textarea.New()
	input.Placeholder = "Send a message..."
	input.Focus()
	input.Prompt = "┃ "
	input.CharLimit = 2048
	input.SetWidth(remainingCols)
	input.SetHeight(inputTextareaHeight)
	input.FocusedStyle.CursorLine = lipgloss.NewStyle()
	input.ShowLineNumbers = false
	input.KeyMap.InsertNewline.SetEnabled(true)

	// Create viewports
	sidebarVP := viewport.New(sidebarVPWidth, sidebarVPHeight)
	messagesVP := viewport.New(remainingCols, messagesVPHeight)

	// Initialize Claude LLM instance
	claudeLLM, initialized := initializeClaudeLLM()
	if !initialized {
		return model{claudeInitErr: fmt.Errorf("failed to initialize Claude LLM")}
	}

	// Welcome message and sidebar content
	messagesVPWelcome := "\n\nWelcome to the chat room!\nType a message and press Enter to send."
	claudeEnterMessage := "Claude has entered the chat"
	botStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Italic(true)
	claudeEnterMsg := botStyle.Render(claudeEnterMessage)

	placeholderContent := "\n\nSidebar Content\nPlaceholder Text\nMore Text..."
	welcomeMessage := []string{messagesVPWelcome, claudeEnterMsg}

	sidebarVP.SetContent(placeholderContent)
	messagesVP.SetContent(strings.Join(welcomeMessage, "\n"))

	// Return model with initial state
	return model{
		sidebarVP:   sidebarVP,
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
			m.input.Reset() // Clear textarea

			userStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Italic(true) // User message style
			m.messages = append(m.messages, userStyle.Render("You: "+userInput))          // Append user message

			botStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5")) // Bot message style

			// Update the viewport with the user's message immediately
			m.messagesVP.SetContent(strings.Join(m.messages, "\n"))
			m.messagesVP.GotoBottom()

			// Call Claude LLM with the user input
			claudeResponse, err := callClaudeLLM(userInput, m.claudeLLM)
			botMsg := ""
			if err != nil {
				botMsg = botStyle.Render("Claude: Error processing your request.")
				m.messages = append(m.messages, botMsg)
			} else {
				botMsg = botStyle.Render("Claude:" + claudeResponse)
				m.messages = append(m.messages, botMsg) // Append Claude's response to messages
			}

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
	// Calculate viewport views
	sidebarView := m.sidebarVP.View()
	messagesView := m.messagesVP.View()
	inputTextareaView := m.input.View()

	// Define styles for the viewports
	sidebarStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.AdaptiveColor{Light: "5", Dark: "5"}).Margin(0, 0)
	messagesStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.AdaptiveColor{Light: "5", Dark: "5"}).Margin(0, 1)
	textareaStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.AdaptiveColor{Light: "5", Dark: "5"}).Margin(0, 1)

	// Apply styles to the viewport content
	styledSidebar := sidebarStyle.Render(sidebarView)
	styledMessages := messagesStyle.Render(messagesView)
	styledTextarea := textareaStyle.Render(inputTextareaView)

	// Adjust the arrangement of views
	layout := lipgloss.JoinVertical(
		lipgloss.Bottom,
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			styledSidebar,
			lipgloss.JoinVertical(lipgloss.Left, styledMessages, styledTextarea), // Rearrange the messages and textarea views
		),
	)

	return layout
}
