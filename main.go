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
	messagesVP    viewport.Model // For displaying chat messages
	inputVP       viewport.Model // For user input
	messages      []string       // Slice of strings to store chat messages
	input         textarea.Model // Textarea component for user input
	senderStyle   lipgloss.Style // Style for rendering user messages
	err           error          // Error field to store any errors
	claudeLLM     *claude.LLM    // Claude LLM instance
	claudeInitErr error          // Error during Claude LLM initialization
}

// Returns the initial model state
func initialModel(cols, rows int) model {
	// Calculate dynamic viewport heights
	messagesVPHeight := rows - 8 // Adjust as needed
	inputVPHeight := 3           // Adjust as needed
	inputTextareaHeight := 3     // Adjust as needed

	// Calculate remaining space for message and input viewports
	remainingRows := rows - (messagesVPHeight + inputVPHeight + inputTextareaHeight)

	// Determine if there's extra space to distribute
	if remainingRows > 0 {
		inputVPHeight += remainingRows
	}

	// Create a new textarea component
	input := textarea.New()
	input.Placeholder = "Send a message..." // Set placeholder text
	input.Focus()                           // Set initial focus on textarea

	input.Prompt = "┃ "    // Set textarea prompt style
	input.CharLimit = 2048 // Set character limit

	// Set textarea dimensions
	input.SetWidth(cols)
	input.SetHeight(inputTextareaHeight)

	input.FocusedStyle.CursorLine = lipgloss.NewStyle() // Remove cursor line styling
	input.ShowLineNumbers = false                       // Disable line number view

	// Create a new viewport with console width and height
	messagesVP := viewport.New(cols, messagesVPHeight)
	messagesVPWelcome := "Welcome to the chat room!\nType a message and press Enter to send." // Set initial welcome message

	// Create a new viewport for the input
	inputVP := viewport.New(cols, inputVPHeight)

	input.KeyMap.InsertNewline.SetEnabled(false) // Disable newline insertion on enter

	// Initialize Claude LLM instance and check if initialization was successful
	claudeLLM, initialized := InitializeClaudeLLM()
	if !initialized {
		// Handle initialization failure
		return model{claudeInitErr: fmt.Errorf("failed to initialize Claude LLM")}
	}

	// Add message indicating Claude has entered the chat
	claudeEnterMessage := "Claude has entered the chat"
	botStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Italic(true)
	claudeEnterMsg := botStyle.Render(claudeEnterMessage)

	welcomeMessage := []string{messagesVPWelcome}
	welcomeMessage = append(welcomeMessage, claudeEnterMsg) // Append Claude's entrance message

	// Set viewport content with the modified messages slice
	messagesVP.SetContent(strings.Join(welcomeMessage, "\n"))

	// Return model with initial state
	return model{
		input:       input,
		messages:    []string{},
		messagesVP:  messagesVP,
		inputVP:     inputVP,
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
			userStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Italic(true) // User message style
			m.messages = append(m.messages, userStyle.Render("You: "+m.input.Value()))    // Append user message

			botStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5")) // Bot message style

			// Call Claude LLM with the user input
			claudeResponse, err := callClaudeLLM(m.input.Value(), m.claudeLLM)
			botMsg := ""
			if err != nil {
				botMsg = botStyle.Render("Claude: Error processing your request.")
				m.messages = append(m.messages, botMsg)
			} else {
				botMsg = botStyle.Render("Claude:" + claudeResponse)
				m.messages = append(m.messages, botMsg) // Append Claude's response to messages
			}

			m.input.Reset()                                         // Clear textarea
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
func InitializeClaudeLLM() (*claude.LLM, bool) {
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
	messagesView := m.messagesVP.View()
	inputView := m.inputVP.View()
	inputTextareaView := m.input.View()

	// Put the textarea inside the input viewport
	// This concatenates the input textarea view with the input viewport view
	fullInputView := inputView + "\n" + inputTextareaView

	// Arrange views without overlap
	fullView := messagesView + "\n\n" + fullInputView

	return fullView
}
