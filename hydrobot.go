package main //Package main comment

//Package main HydroBot is a simple Go application that provides hydration reminders.
// It uses Fyne for the GUI and communicates via channels. For now.
//Used AI in Socratic mode.

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"log"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var hydrationTimer *time.Timer
var timerMu sync.Mutex            // Mutex to protect access to hydrationTimer's Reset method, which is not concurrently safe
var reminderMessages []string     // Slice to store messages loaded from reminders.txt
var escalationMessages []string   // Slice to store messages loaded from escalations.txt
var confirmationMessages []string //same as above
var version = "dev"               // Global variable for version, can be set by linker flags

// Constants defining the intervals, keywords, and file paths for the bot.
const (
	normalInterval        = 30 * time.Minute  // Default reminder interval (increased for stability)
	elevatedInterval      = 10 * time.Minute  // Shorter interval when escalation occurs (increased for stability)
	confirmationKeyword   = "agua"            // Keyword user types to confirm hydration
	remindersFilePath     = "reminders.txt"   // Path to the file containing reminder messages
	escalationsFilePath   = "escalations.txt" // Path to the file containing escalation messages
	confirmationsFilePath = "confirmations.txt"
)








func main() {

	// Seed the random number generator.
	// for getRandomMessage
	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)

	// Create channels for communication between goroutines.
	// confirmationChan: Used by readUserInput to signal hydration confirmation to manageReminders.
	// adminCommandChan: Used by readUserInput to send admin commands (on, off, debug) to manageReminders.
	confirmationChan := make(chan struct{})
	adminCommandChan := make(chan string)
	GUIChan := make(chan string)

	var err error
	reminderMessages, err = loadMessagesFromFile(remindersFilePath)
	if err != nil {
		fmt.Printf("Warning: error loading reminder messages: %v. Using default messages.\n", err)
		reminderMessages = []string{"Time to drink some water!", "Hydration check!"}
	}

	escalationMessages, err = loadMessagesFromFile(escalationsFilePath)
	if err != nil {
		fmt.Printf("Warning: error loading escalation messages: %v. Using default messages.\n", err)
		escalationMessages = []string{"Urgent: Drink NOW!", "Immediate hydration required!"}
	}

	confirmationMessages, err = loadMessagesFromFile(confirmationsFilePath)
	if err != nil {
		fmt.Printf("Warning: error loading confirmation messages: %v. Using default messages.\n", err)
		escalationMessages = []string{"Kipik est content, tu bois bien!", "Koya aussi est content que tu boives."}
	}

	go manageReminders(confirmationChan,
		adminCommandChan,
		GUIChan,
		normalInterval,
		elevatedInterval,
		confirmationMessages,
		rng)

	startGUI(GUIChan, confirmationChan, adminCommandChan)

}


func sendNotification(title, message string) {
	// The notify-send command is the standard way to send desktop notifications
	// on Linux. It will automatically play a sound if configured in the desktop environment.
	cmd := exec.Command("notify-send", title, message)
	
	// Run the command. If there's an error, log it.
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to send notification: %v", err)
	}
}

func loadMessagesFromFile(filePath string) ([]string, error) {
	var messages []string

	file, err := os.Open(filePath)
	if err != nil {

		return nil, fmt.Errorf("error opening file %s: %w", filePath, err)
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			messages = append(messages, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages found in %s", filePath)
	}

	return messages, nil
}

func getRandomMessage(messages []string, rng *rand.Rand) string {
	messagesCount := len(messages)

	if messagesCount == 0 {
		return "No messages available."
	}

	// Use the provided 'rng' instance to generate the random index
	randomIndex := rng.Intn(messagesCount)

	return messages[randomIndex]
}

func startGUI(GUIChan chan string, confirmationChan chan struct{}, adminCommandChan chan<- string) {
	a := app.New()
	w := a.NewWindow("HydroBot")
	w.Resize(fyne.NewSize(1200, 800))
	icon, err := fyne.LoadResourceFromPath("Icon.png")
	if err != nil {
		fmt.Printf("Warning: could not load icon.png. %v\n", err)
	} else {
		w.SetIcon(icon)
	}

	welcomeLabel := "Welcome to GlouglouLand. I'll help you not keep dry at all!"
	reminderWidget := widget.NewLabel(welcomeLabel)

	botTalkArea := container.NewVBox(
		reminderWidget)

	scrollArea := container.NewScroll(botTalkArea)

	go func() {
		for msg := range GUIChan {

			fyne.Do(func() {
				newMessage := widget.NewLabel(msg)
				botTalkArea.Add(newMessage)
				botTalkArea.Refresh()
				scrollArea.ScrollToBottom()
			})
		}
	}()

	input := widget.NewEntry()
	input.MultiLine = false

	onSend := func(submittedText string) {
		trimmedInput := strings.TrimSpace(submittedText)
		loweredInput := strings.ToLower(trimmedInput)

		if strings.HasPrefix(loweredInput, "!") {
			command, _ := strings.CutPrefix(loweredInput, "!")
			command = strings.TrimSpace(command)

			switch command {
			case "on":
				GUIChan <- "Admin: Command 'on' received. Activating bot."
				adminCommandChan <- command
			case "off":
				GUIChan <- "Admin: Command 'off' received. Pausing bot."
				adminCommandChan <- command
			case "debug":
				GUIChan <- "Admin: Command 'debug' received. Requesting debug info."
				adminCommandChan <- command
			case "other":
				GUIChan <- "Admin: Command 'other' received."
				adminCommandChan <- command
			default:
				GUIChan <- fmt.Sprintf("Admin: Unrecognized command '%s'.", command)
			}
		} else if loweredInput == confirmationKeyword {

			confirmationChan <- struct{}{}
		} else {
			GUIChan <- fmt.Sprintf("Unrecognized input '%s'. Please type '%s' to confirm, or a command starting with '!'.", trimmedInput, confirmationKeyword)
		}

		input.SetText("") // Clear the input field after sending
	}
	// HydroBot is a simple Go application that provides hydration reminders.
	// It uses Fyne for the GUI and communicates via channels.
	// --- Assign the reusable function to both the button and the input field ---
	sendButton := widget.NewButton("[send]", func() {
		onSend(input.Text) // <-- Button calls onSend with current text
	})
	input.OnSubmitted = onSend // Input field calls onSend when Enter is pressed

	inputArea := container.NewBorder(
		nil,
		nil,
		nil,
		sendButton,
		input,
	)

	mainLayout := container.NewBorder(
		nil,
		inputArea,
		nil,
		nil,
		scrollArea)

	w.SetContent(mainLayout)
	w.ShowAndRun()
}

func manageReminders(
	confirmationChan <-chan struct{},
	adminCommandChan <-chan string,
	GUIChan chan<- string,
	normalInterval time.Duration,
	elevatedInterval time.Duration,
	confirmationMessages []string,
	rng *rand.Rand) {

	currentInterval := normalInterval // Tracks the current reminder frequency
	escalation := false               // Flag: true if bot is in escalation mode
	alive := true                     // Flag: true if bot is actively sending reminders (can be toggled by !on/!off)

	hydrationTimer = time.NewTimer(normalInterval)

	for {
		select {
		case <-confirmationChan:

			timerStopped := hydrationTimer.Stop()
			//GUIChan <- fmt.Sprintf("Debug: Timer stop status: %t", timerStopped)

			if !timerStopped {
				select {
				case <-hydrationTimer.C:
					//GUIChan <- "Debug: Drained a pending timer tick."
				default:
					// No pending tick, or it was already drained.
				}
			}
			GUIChan <- getRandomMessage(confirmationMessages, rng)
			currentInterval = normalInterval     // Reset interval to normal
			timerMu.Lock()                       // Acquire mutex before resetting shared timer
			hydrationTimer.Reset(normalInterval) // Restart timer for the normal interval
			timerMu.Unlock()                     // Release mutex
			escalation = false                   // Exit escalation mode
			alive = true                         // Confirmation implies bot should be active

		case command := <-adminCommandChan:
			switch command {
			case "on":
				timerMu.Lock()
				hydrationTimer.Reset(normalInterval)
				timerMu.Unlock()
				GUIChan <- "Hello! GlouglouBot is now active."
				alive = true
			case "off":
				hydrationTimer.Stop()
				GUIChan <- "GlouglouBot is going to sleep. No more reminders for now."
				alive = false
			case "debug":
				GUIChan <- fmt.Sprintf("HydroBot, version %s. Status: Alive=%t, Escalation=%t, Current Interval=%v.\n",
					version, alive, escalation, currentInterval)
			case "other":
				GUIChan <- "manageReminders: Received 'other' command. (No specific action defined yet)"
			default:
				GUIChan <- "manageReminders: Received an unhandled admin command."
			}

		case <-hydrationTimer.C:
			if alive {
				if escalation {
					hydrationTimer.Stop()
					msg := getRandomMessage(escalationMessages, rng)
          sendNotification("HydroBot Escalation!", msg)
					GUIChan <- msg
					currentInterval = elevatedInterval
					timerMu.Lock()
					hydrationTimer.Reset(elevatedInterval)
					timerMu.Unlock()

				} else {
					msg := getRandomMessage(reminderMessages, rng) 
          sendNotification("💧 Time to Hydrate!", msg)
					GUIChan <- msg
					hydrationTimer.Stop()
					currentInterval = elevatedInterval
					timerMu.Lock()
					hydrationTimer.Reset(elevatedInterval)
					timerMu.Unlock()
					escalation = true
				}
			} else {
				timerMu.Lock()
				hydrationTimer.Reset(currentInterval)
				timerMu.Unlock()
			}
		}
	}
}
