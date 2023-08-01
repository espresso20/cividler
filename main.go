package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	dbFile       = "game.db"
	bucketName   = "GameState"
	flintKeyName = "FlintCount"
)

type Game struct {
	resourceMap map[string]*Resource
	done        chan bool
	db          *bolt.DB
}

type Resource struct {
	Name  string
	Count int
	Rate  time.Duration
	// Ticker *time.Ticker
	// StartTime time.Time
	LastUpdated time.Time
}

func (g *Game) startGatheringResources() {
	// for _, resource := range g.resourceMap {
	// 	resource.StartTime = time.Now()
	// 	go func(res *Resource) {
	// 		ticker := time.NewTicker(res.Rate)
	// 		defer ticker.Stop()

	// 		for {
	// 			select {
	// 			case <-ticker.C:
	// 				res.Count++
	// 				g.saveState()
	// 			case <-g.done:
	// 				return
	// 			}
	// 		}
	// 	}(resource)
	// }
	for _, resource := range g.resourceMap {
		resource.LastUpdated = time.Now()
	}
}

func (r *Resource) currentCount() int {
	elapsed := time.Since(r.LastUpdated).Seconds()
	increment := int(elapsed / r.Rate.Seconds())
	return r.Count + increment
}

//	func (g *Game) calculateProduction() {
//		camps := g.resourceMap["camp"].Count
//		g.resourceMap["villager"].Count += camps
//	}
func (g *Game) buyCamps(numCamps int) {
	villagers := g.resourceMap["villager"]
	totalCost := numCamps * 50
	if villagers.currentCount() >= totalCost {
		villagers.Count = villagers.currentCount() - totalCost
		villagers.LastUpdated = time.Now()
		camp := g.resourceMap["camp"]
		camp.Count = camp.currentCount() + numCamps
		camp.LastUpdated = time.Now()
		fmt.Printf("You bought %d new camp(s). Now your civilization produces %d villagers per second.\n", numCamps, camp.Count)
	} else {
		fmt.Printf("You need at least %d villagers to buy %d new camp(s).\n", totalCost, numCamps)
	}
}

func (g *Game) loadState() {
	g.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))

		// Get the serialized map from the database
		v := b.Get([]byte("ResourceMap"))

		if v != nil {
			// Create a bytes.Buffer with the data and a gob.Decoder for it
			buf := bytes.NewBuffer(v)
			dec := gob.NewDecoder(buf)

			// Decode the buffer into the map
			err := dec.Decode(&g.resourceMap)
			if err != nil {
				return fmt.Errorf("failed to deserialize resource map: %v", err)
			}

			// Calculate how much time has passed for each resource and update its Count
			for _, res := range g.resourceMap {
				timePassed := time.Since(res.LastUpdated)
				increases := int(timePassed / res.Rate)
				res.Count += increases
				res.LastUpdated = time.Now() // reset LastUpdated time to now
			}
		} else {
			// If there's no data in the database, initialize an empty map
			g.resourceMap = make(map[string]*Resource)
			g.resourceMap["camp"] = &Resource{Name: "camp", Count: 1, Rate: 10 * time.Second, LastUpdated: time.Now()}
			g.resourceMap["villager"] = &Resource{Name: "villager", Count: 0, Rate: 5 * time.Second, LastUpdated: time.Now()}
		}
		return nil
	})
}

func (g *Game) saveState() {
	g.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))

		// Update the LastUpdated field for each resource
		for _, res := range g.resourceMap {
			res.LastUpdated = time.Now()
		}

		// create a bytes.Buffer to hold the serialized map of resources
		buf := new(bytes.Buffer)
		//create a gob.Encoder and ecode the map of resources into the buffer
		enc := gob.NewEncoder(buf)

		// binary.BigEndian.PutUint64(buf, uint64(g.flintCount))
		// err := b.Put([]byte(flintKeyName), buf)
		err := enc.Encode(g.resourceMap)
		if err != nil {
			return fmt.Errorf("failed to seialize the resource map: %v", err)
		}

		// put the serialized map into the database
		return b.Put([]byte("ResourceMap"), buf.Bytes())
	})
}

func NewGame(db *bolt.DB) (*Game, error) {
	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})

	game := &Game{
		db:   db,
		done: make(chan bool),
		resourceMap: map[string]*Resource{
			"camp":     {Name: "camp", Count: 1, Rate: 10 * time.Second},
			"villager": {Name: "villager", Count: 0, Rate: 20 * time.Millisecond},
		},
	}

	game.loadState()

	game.startGatheringResources()

	go func() {
		for {
			time.Sleep(time.Second) // update every second
			now := time.Now()
			for _, res := range game.resourceMap {
				timePassed := now.Sub(res.LastUpdated)
				increases := int(timePassed / res.Rate)
				res.Count += increases
				res.LastUpdated = now
			}
			game.saveState() // save game state after updating resources
		}
	}()

	return game, nil
}

func main() {
	reader := bufio.NewReader(os.Stdin)

	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		fmt.Println("Failed to open the Database:", err)
		return
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

	for {
		fmt.Print("Enter command: ")
		text, _ := reader.ReadString('\n')

		// Split the input command
		commandParts := strings.Fields(text)
		command := ""
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
				var err error
				numCampsToBuy, err = strconv.Atoi(commandParts[1])
				if err != nil {
					fmt.Println("Invalid number of camps to buy. Please enter a valid number.")
					continue
				}
			}

			game.buyCamps(numCampsToBuy)

		case "exit", "e":
			// if the user imputs "exit", save th state and exit the game
			fmt.Println("Saving and Exiting the game...")
			game.saveState()
			os.Exit(0)
		case "resources", "r":
			fmt.Println("Resources:")
			var availableCamps, totalCost int
			for _, resource := range game.resourceMap {
				fmt.Printf("|%s|%d|\n", resource.Name, resource.Count)
				if resource.Name == "villager" {
					availableCamps = resource.Count / 50
					totalCost = availableCamps * 50
				}
			}
			if availableCamps > 0 {
				fmt.Printf("You can buy %d camp(s) for a total cost of %d villagers.\n", availableCamps, totalCost)
			}
		case "help":
			fmt.Println("Commands")
			fmt.Println("Commands:")
			fmt.Println("Type \"bc\" to buy more camps. This will increase the rate at which you create villagers!")
			fmt.Println("Type \"exit\" or \"e\" to save and cleanly exit the game.")
			fmt.Println("Type \"resources\" or \"r\" to see your current resource counts.")
			fmt.Println("Type \"help\" to display this help message.")
		default:
			fmt.Println("Invalid command. Type \"help\" for a list of valid commands.")
		}
	}
}
