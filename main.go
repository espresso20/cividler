package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dgraph-io/badger/v4"
)

const (
	CampProductionRate = 1
	TownProductionRate = 2
)

type Game struct {
	resourceMap map[string]*Resource
	done        chan bool
	db          *badger.DB
	ticker      *time.Ticker
}

type Resource struct {
	Name        string
	Description string
	Cost        int
	Count       int
	BaseRate    time.Duration
	Rate        time.Duration
	LastUpdated time.Time
	TownEnabled bool
}

func (g *Game) startGatheringResources() {
	for _, resource := range g.resourceMap {
		resource.LastUpdated = time.Now()

		// Start a goroutine for eacch resource to increase over time
		go func(res *Resource) {
			for {
				time.Sleep(res.Rate)
				res.Count = res.currentCount()
				res.LastUpdated = time.Now()

				// Save state after updating resource
				err := g.saveState()
				if err != nil {
					log.Printf("Error while saving state: %v\n", err)
					return
				}

				// If camps reach 500 for the first time, allow the purchase of towns
				if res.Name == "camp" && res.Count >= 500 && !res.TownEnabled {
					fmt.Println("Your villagers have found clay, and can build bricks with which to create a town. Towns are now available for purchase")
					res.TownEnabled = true
				}
			}
		}(resource)
	}
}

func (r *Resource) currentCount() int {
	elapsed := time.Since(r.LastUpdated).Seconds()
	increment := int(elapsed / r.Rate.Seconds())
	return r.Count + increment
}

func (g *Game) buyCamps(quantity int) {
	// Calculate cost of new camps
	cost := quantity * 50
	villagers := g.resourceMap["villager"]
	if villagers.Count < cost {
		fmt.Printf("You need at least %d villagers to buy %d new camp(s).\n", cost, quantity)
		return
	}

	// Deduct cost from villagers
	villagers.Count -= cost
	villagers.LastUpdated = time.Now()

	// Add new camps
	camps := g.resourceMap["camp"]
	camps.Count += quantity
	camps.LastUpdated = time.Now()

	totalVillagersPerSecond := g.resourceMap["camp"].Count*CampProductionRate + g.resourceMap["town"].Count*TownProductionRate
	fmt.Printf("You bought %d new camp(s). Now your civilization produces %d villagers per second.\n", quantity, totalVillagersPerSecond)

}

func (g *Game) buyTowns(quantity int) {
	cost := quantity * 125
	villagers := g.resourceMap["villager"]
	camps := g.resourceMap["camp"]

	// Check for the total of camps required to get your first town
	if camps.Count < 500 {
		fmt.Println("Towns are not available for purchase yet. you need at least 500 Camps to purchase a Town!")
		return
	}

	if villagers.Count < cost {
		fmt.Printf("You need at least %d villager to buy %d new town(s).\n", cost, quantity)
		return
	}

	villagers.Count -= cost
	villagers.LastUpdated = time.Now()

	towns := g.resourceMap["town"]
	towns.Count += quantity
	towns.LastUpdated = time.Now()

	totalVillagersPerSecond := g.resourceMap["camp"].Count*CampProductionRate + g.resourceMap["town"].Count*TownProductionRate
	fmt.Printf("You bought %d new town(s). Your Civilization now produces %d villagers per second.\n", quantity, totalVillagersPerSecond)

}
func (g *Game) loadState() {
	err := g.db.Update(func(txn *badger.Txn) error {
		for resourceName := range g.resourceMap {
			item, err := txn.Get([]byte(resourceName))
			if err != nil {
				if err == badger.ErrKeyNotFound {
					// If the key is not found in the database, initialize it with a default value.
					res := &Resource{Name: resourceName, Count: 0, Rate: 10 * time.Second, LastUpdated: time.Now()}
					g.resourceMap[resourceName] = res

					// Save the default resource into the database.
					var buf bytes.Buffer
					enc := gob.NewEncoder(&buf)
					if err := enc.Encode(res); err != nil {
						return fmt.Errorf("error when trying to encode resource %s: %v", resourceName, err)
					}

					if err := txn.Set([]byte(resourceName), buf.Bytes()); err != nil {
						return fmt.Errorf("error when trying to save resource %s: %v", resourceName, err)
					}

					continue
				} else {
					return fmt.Errorf("error when trying to get resource %s: %v", resourceName, err)
				}
			}

			err = item.Value(func(val []byte) error {
				log.Println("Start decoding resource", resourceName)

				buf := bytes.NewBuffer(val)
				dec := gob.NewDecoder(buf)

				var res Resource
				err := dec.Decode(&res)
				if err != nil {
					return fmt.Errorf("error when trying to decode resource %s: %v", resourceName, err)
				}

				log.Println("Finished decoding resource", resourceName)

				g.resourceMap[resourceName] = &res

				// Calculate how much time has passed for each resource and update its Count
				timePassed := time.Since(res.LastUpdated)
				if res.Rate != 0 {
					increases := int(timePassed.Seconds() / res.Rate.Seconds())
					res.Count += increases
				} else {
					log.Printf("Warning: Rate for resource %s is 0, not updating Count", resourceName)
				}
				res.LastUpdated = time.Now() // reset LastUpdated time to now

				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		log.Fatalf("Failed to load state: %v", err)
	}
}

func (g *Game) saveState() error {
	return g.db.Update(func(txn *badger.Txn) error {
		for resourceName, resource := range g.resourceMap {
			buf := new(bytes.Buffer)
			enc := gob.NewEncoder(buf)

			err := enc.Encode(resource)
			if err != nil {
				return fmt.Errorf("error when trying to encode resource %s: %v", resourceName, err)
			}

			err = txn.Set([]byte(resourceName), buf.Bytes())
			if err != nil {
				return fmt.Errorf("error when trying to save resource %s: %v", resourceName, err)
			}
		}

		return nil
	})

}
func NewGame(db *badger.DB) (*Game, error) {
	game := &Game{
		db:   db,
		done: make(chan bool),
		resourceMap: map[string]*Resource{
			"villager": {
				Name:        "villager",
				Description: "Villagers help your civilization to grow.",
				Cost:        0,
				Count:       0,
				BaseRate:    1 * time.Second,
				Rate:        1 * time.Second,
				LastUpdated: time.Now(),
				TownEnabled: false,
			},
			"camp": {
				Name:        "camp",
				Description: "Camps produce villagers and allow your civilization to grow.",
				Cost:        50,
				Count:       1,
				BaseRate:    1 * time.Second,
				Rate:        1 * time.Second,
				LastUpdated: time.Now(),
				TownEnabled: false,
			},
			"town": {
				Name:        "town",
				Description: "Towns are an advanced way to grow your civilization and produce more villagers.",
				Cost:        500,
				Count:       0,
				BaseRate:    2 * time.Second,
				Rate:        2 * time.Second,
				LastUpdated: time.Now(),
				TownEnabled: false,
			},
		},
	}

	game.loadState()

	go func() {
		game.ticker = time.NewTicker(1 * time.Second) // set up a ticker to update every second
		for {
			select {
			case <-game.done:
				game.ticker.Stop()
				return
			case <-game.ticker.C:
				now := time.Now()
				camps := game.resourceMap["camp"]
				towns := game.resourceMap["town"]
				villagers := game.resourceMap["villager"]
				villagers.Count += camps.Count*CampProductionRate + towns.Count*TownProductionRate // Increment villagers count by the number of camps and towns
				villagers.LastUpdated = now
				err := game.saveState() // save game state after updating resources
				if err != nil {
					log.Printf("Error while saving state: %v\n", err)
					return
				}
			}
		}
	}()

	return game, nil
}

func main() {
	// Open BadgerDB
	dbFile := "gameDB" // define dbFile
	opts := badger.DefaultOptions(dbFile)
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	game, err := NewGame(db)
	if err != nil {
		fmt.Println("Failed to initialize the game:", err)
		return
	}

	game.loadState()

	game.startGatheringResources()

	fmt.Println("Welcome to Cividler! The CLI based idle game.")
	fmt.Println("Commands:")
	fmt.Println("Type \"help\" to see available commands.")

	// Handle a Ctrl+C event
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		// game.stopFindingFlint()
		os.Exit(0)
	}()

	// reader := bufio.NewReader(os.Stdin) // define reader

	for {
		// Print the game's status
		fmt.Printf("Villagers: %d\n", game.resourceMap["villager"].Count)
		fmt.Printf("Camps: %d\n", game.resourceMap["camp"].Count)
		fmt.Printf("Towns: %d\n", game.resourceMap["town"].Count)

		// Wait for the user's command
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter command: ")
		text, _ := reader.ReadString('\n')

		// Split the input command
		commandParts := strings.Fields(text) // define commandParts
		command := ""                        // define command
		if len(commandParts) > 0 {
			command = commandParts[0]
		}

		switch command {
		case "bc":
			if len(commandParts) < 2 {
				fmt.Println("Invalid command. Format should be 'bc <number>' or 'bc all'.")
				continue
			}
			var numCampsToBuy int

			if commandParts[1] == "all" {
				numCampsToBuy = game.resourceMap["villager"].Count / 50
			} else {
				numCampsToBuy, err = strconv.Atoi(commandParts[1])
				if err != nil {
					fmt.Println("Invalid number of camps to buy. Please enter a valid number.")
					continue
				}
			}

			game.buyCamps(numCampsToBuy)

		case "bt":
			if len(commandParts) < 2 {
				fmt.Println("Invalid command. Format should be 'bt <number>' or 'bt all'.")
				continue
			}
			var numTownsToBuy int

			if commandParts[1] == "all" {
				numTownsToBuy = game.resourceMap["villager"].Count / 125
			} else {
				numTownsToBuy, err = strconv.Atoi(commandParts[1])
				if err != nil {
					fmt.Println("Invalid number of Towns to buy. Please enter a valid number.")
					continue
				}
			}

			game.buyTowns(numTownsToBuy)

		case "exit", "e":
			// if the user imputs "exit", save th state and exit the game
			fmt.Println("Saving and Exiting the game...")
			game.done <- true
			err := game.saveState()
			if err != nil {
				log.Printf("Error while saving state: %v\n", err)
				return
			}
			os.Exit(0)

		case "resources", "r":
			fmt.Println("Resources:")
			var availableCamps, availableTowns, totalCampCost, totalTownCost int
			for _, resource := range game.resourceMap {
				fmt.Printf("|%s|%d|\n", resource.Name, resource.Count)
				if resource.Name == "villager" {
					availableCamps = resource.Count / 50
					availableTowns = resource.Count / 125
					totalCampCost = availableCamps * 50
					totalTownCost = availableTowns * 125
				}
			}
			if availableCamps > 0 {
				fmt.Printf("You can buy %d camp(s) for a total cost of %d villagers.\n", availableCamps, totalCampCost)
			}
			if availableTowns > 0 {
				fmt.Printf("You can also buy %d town(s) for a total cost of %d villagers.\n", availableTowns, totalTownCost)
			}

		case "help", "h":
			fmt.Println("Commands:")
			fmt.Println("Type \"bc\" to buy more camps. This will increase the rate at which you create villagers!")
			fmt.Println("Type \"bt\" to buy more towns. This will increase the rate at which you create villagers!")
			fmt.Println("Type \"exit\" or \"e\" to save and cleanly exit the game.")
			fmt.Println("Type \"resources\" or \"r\" to see your current resource counts.")
			fmt.Println("Type \"help\" to display this help message.")

		default:
			fmt.Println("Invalid command. Type \"help\" for a list of valid commands.")
		}
	}
}
