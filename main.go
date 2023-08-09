package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
)

type Camp struct {
	Count int
}

type Village struct {
	Count int
}

type Player struct {
	Villagers int
	Camps     Camp
	Villages  Village
}

type Game struct {
	Player             Player       `json:"player"`
	Ticker             *time.Ticker `json:"-"`
	LastSaved          time.Time    `json:"last_saved"`
	HasNotifiedCamp    bool         `json:"-"`
	HasNotifiedVillage bool         `json:"-"`
	JustReset          bool         `json:"-"` // add reset flag to game struct
	Done               chan bool    `json:"-"` // use "-" json:tag to tell the JSON package to ignore the channel during serialization
}

type GameSerializable struct {
	Player             Player    `json:"player"`
	LastSaved          time.Time `json:"last_saved"`
	JustReset          bool      `json:"-"` // add reset flag to game struct
	HasNotifiedCamp    bool      `json:"-"`
	HasNotifiedVillage bool      `json:"-"`
}

func NewGame() *Game {
	// try to load the game, so we don't start with a new game every start
	game, err := LoadGame()
	if err != nil {
		// if our loading fails, create a new game state
		game = &Game{
			Player: Player{
				Villagers: 0,
				Camps:     Camp{Count: 1},
				Villages:  Village{Count: 0},
			},
			Ticker: time.NewTicker(1 * time.Second),
		}
		fmt.Println("Starting a new game...")

		// Save the new game immediately
		if err := SaveGame(game); err != nil {
			fmt.Printf("Error saving new game: %v\n", err)
		} else {
			fmt.Println("Initial game state saved!")
		}
	} else {
		fmt.Println("Loaded saved game!")
	}

	game.Done = make(chan bool)
	return game
}

func (g *Game) Run() {
	for {
		select {
		case <-g.Done:
			return
		case <-g.Ticker.C:
			g.Player.Villagers += g.Player.Camps.Count + (g.Player.Villages.Count * 3)

			// Notify the player whenever they have enough villagers to buy a new camp
			if g.Player.Villagers >= 50 && !g.HasNotifiedCamp {
				fmt.Println("You can buy a new camp!")
				g.HasNotifiedCamp = true
			}

			// You can set a condition for notifying about the village purchase if you want
			// For now, I'm assuming once they have bought 500 camps, they'll be notified about the village
			if g.Player.Camps.Count == 500 && !g.HasNotifiedVillage {
				fmt.Println("You can now purchase a village!")
				g.HasNotifiedVillage = true
			}
		}
	}
}

// Some vanity stuff for dashboards
// Center a string within a given width using spaces
func centerString(s string, width int) string {
	if len(s) >= width {
		return s
	}
	spaces := (width - len(s)) / 2
	return strings.Repeat(" ", spaces) + s + strings.Repeat(" ", width-len(s)-spaces)
}

// playing with dynamic formatting
func printHelpMenu() {
	commands := map[string]string{
		"help (h)":         "Provides help context on commands. Some commands have short alias versions.",
		"exit":             "Exits the game after saving.",
		"load":             "Loads a game from memory.",
		"save":             "Saves your current game to memory. This should be run immediately after using reset",
		"reset":            "Resets your entire game, no turning back from this one!",
		"status (s)":       "Shows a dashboard of current resources.",
		"buy camp (bc)":    "Buy a camp.",
		"buy village (bv)": "Buy a village.",
	}

	maxWidthCommand := 0
	maxWidthDescription := 0
	for cmd, desc := range commands {
		if len(cmd) > maxWidthCommand {
			maxWidthCommand = len(cmd)
		}
		if len(desc) > maxWidthDescription {
			maxWidthDescription = len(desc)
		}
	}

	totalWidth := maxWidthCommand + maxWidthDescription + 7 // 7 = "|" + " " + "|" + " " + "|" + " " + "|"

	header := centerString("Cividler Help Commands", totalWidth-4) // -4 for the border
	cyan := color.New(color.FgCyan).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	fmt.Println(cyan("+" + strings.Repeat("-", totalWidth-2) + "+"))
	fmt.Println(cyan("| ") + yellow(header) + cyan(" |"))
	fmt.Println(cyan("+" + strings.Repeat("-", totalWidth-2) + "+"))

	for cmd, desc := range commands {
		fmt.Printf(cyan("| ")+yellow("%-"+fmt.Sprint(maxWidthCommand)+"s ")+cyan("| ")+"%-"+fmt.Sprint(maxWidthDescription)+"s "+cyan("|")+"\n", cmd, desc)
	}

	fmt.Println(cyan("+" + strings.Repeat("-", totalWidth-2) + "+"))
}

func handleCommands(g *Game) {
	reader := bufio.NewReader(os.Stdin)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		cmd := scanner.Text()

		// Check if command starts with "buy camp"
		if strings.HasPrefix(cmd, "buy camp") {
			fmt.Println("Debug: Inside 'buy camp' command handler")
			// cyan := color.New(color.FgCyan).SprintFunc()
			yellow := color.New(color.FgYellow).SprintFunc()

			parts := strings.Split(cmd, " ")
			var numCamps int
			if len(parts) == 3 {
				if parts[2] == "all" {
					// This should determine the number of camps the player can buy based on the number of villagers they have.
					numCamps = g.Player.Villagers / 50
				} else {
					var err error
					numCamps, err = strconv.Atoi(parts[2])
					if err != nil || numCamps <= 0 {
						fmt.Println("Invalid number of camps")
						continue
					}
				}
			} else {
				numCamps = 1
			}

			// Add a debug log here to understand the number of camps being computed
			fmt.Printf("Debug: Trying to buy %d camps\n", numCamps)

			totalCost := numCamps * 50
			if g.Player.Villagers >= totalCost {
				g.Player.Villagers -= totalCost
				g.Player.Camps.Count += numCamps
				if numCamps == 1 {
					fmt.Println("Bought a camp!")
				} else {
					fmt.Printf("Bought %d camps!\n", numCamps)
				}
			} else {
				fmt.Println(yellow("Cannot buy the specified number of camps right now!"))
				// Another debug log for better understanding
				fmt.Printf("Debug: Current villagers: %d, Total cost: %d, Current + intended camps: %d\n", g.Player.Villagers, totalCost, g.Player.Camps.Count+numCamps)
			}
			continue
		}

		switch cmd {
		case "help", "h":
			printHelpMenu()

		case "exit", "quit":
			fmt.Println("Exiting CivIdler, your game progress has been saved")
			fmt.Println("Sending done signal") //debug message
			g.Done <- true
			g.Ticker.Stop()               //Stop the ticker explicitly to try and catch the done signal directly
			fmt.Println("ticker stopped") //debug message
			if err := SaveGame(g); err != nil {
				fmt.Printf("Error saving game: %v\n", err)
			} else {
				fmt.Println("Game Saved!")
			}
			return

		case "buy village", "bv":
			cyan := color.New(color.FgCyan).SprintFunc()
			yellow := color.New(color.FgYellow).SprintFunc()
			if g.Player.Camps.Count == 500 && g.Player.Villagers >= 250 {
				g.Player.Villagers -= 250
				g.Player.Villages.Count++
				fmt.Println("Bought a village!")
			} else {
				fmt.Println(yellow("Cannot buy a village right now, you need at least"), cyan("250"), yellow("Villagers!"))
			}
		case "status", "s":
			header := centerString("CivIdler", 21)
			cyan := color.New(color.FgCyan).SprintFunc()
			yellow := color.New(color.FgYellow).SprintFunc()

			fmt.Println(cyan("+-----------------------+"))
			fmt.Println(cyan("|"), yellow(header), cyan("|"))
			fmt.Println(cyan("+-----------------------+"))
			fmt.Printf("| Villagers | %9d |\n", g.Player.Villagers)
			fmt.Printf("| Camps     | %9d |\n", g.Player.Camps.Count)
			fmt.Printf("| Villages  | %9d |\n", g.Player.Villages.Count)
			fmt.Println(cyan("+-----------------------+"))
		case "save":
			if err := SaveGame(g); err != nil {
				fmt.Printf("Error saving game: %v\n", err)
			} else {
				fmt.Println("Game saved!")
			}
		case "load":
			loadedGame, err := LoadGame()
			if err != nil {
				fmt.Printf("Error loading game: %v\n", err)
			} else {
				g.Player = loadedGame.Player
				fmt.Println("Game loaded!")
			}
		case "reset":
			yellow := color.New(color.FgYellow).SprintFunc()
			red := color.New(color.FgRed).SprintFunc()
			fmt.Println(red("WARNING:"), yellow("This will completely wipe your current game state and cannot be undone!"))
			fmt.Println(yellow("Make sure to run"), red("save"), yellow("after you have reset, to start a new game!"))
			fmt.Print("Are you sure you want to reset? (yes/no): ")

			response, _ := reader.ReadString('\n')
			response = strings.ToLower(strings.TrimSpace(response))

			if response == "yes" {
				newGame, err := ResetGame()
				if err != nil {
					fmt.Printf("Error resetting game: %v\n", err)
				} else {
					fmt.Println("Game reset successfully!")

					// Update the current game state
					*g = *newGame
				}
			} else {
				fmt.Println("Reset aborted!")
			}

		}
	}
}

func SaveGame(g *Game) error {
	g.LastSaved = time.Now()

	gameSerializable := &GameSerializable{
		Player:             g.Player,
		LastSaved:          g.LastSaved,
		HasNotifiedCamp:    g.HasNotifiedCamp,
		HasNotifiedVillage: g.HasNotifiedVillage,
	}

	data, err := json.Marshal(gameSerializable)
	if err != nil {
		return err
	}

	g.JustReset = false

	return os.WriteFile("gamestate.json", data, 0644)
}

func LoadGame() (*Game, error) {
	data, err := os.ReadFile("gamestate.json")
	if err != nil {
		return nil, err
	}

	var gameSerializable GameSerializable
	err = json.Unmarshal(data, &gameSerializable)
	if err != nil {
		return nil, err
	}

	// Print when the game was last saved
	fmt.Printf("Last saved: %s\n", gameSerializable.LastSaved.Format("2006-01-02 15:04:05"))

	// Create a Game instance and populate it from the deserialized data
	game := &Game{
		Player:             gameSerializable.Player,
		Ticker:             time.NewTicker(1 * time.Second),
		Done:               make(chan bool),
		LastSaved:          gameSerializable.LastSaved,
		HasNotifiedCamp:    gameSerializable.HasNotifiedCamp,
		HasNotifiedVillage: gameSerializable.HasNotifiedVillage,
	}

	if !game.JustReset {
		elapsedSeconds := time.Now().Sub(gameSerializable.LastSaved).Seconds()
		game.Player.Villagers += int(elapsedSeconds) * (game.Player.Camps.Count + (game.Player.Villages.Count * 3))
	}

	return game, nil
}

func ResetGame() (*Game, error) {
	err := os.Remove("gamestate.json")
	if err != nil {
		return nil, err
	}

	// Create a fresh game state
	newGame := &Game{
		Player: Player{
			Villagers: 0,
			Camps:     Camp{Count: 1},
			Villages:  Village{Count: 0},
		},
		Ticker: time.NewTicker(1 * time.Second),
		Done:   make(chan bool),
	}

	return newGame, nil
}

func main() {
	cyan := color.New(color.FgCyan).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	game := NewGame()
	go game.Run()
	fmt.Println("Welcome to CivIdler!")
	fmt.Println(yellow("Type"), cyan("help"), yellow("to see a list of commands"))
	handleCommands(game)
}
