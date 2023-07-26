package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"os/signal"
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
	resourceMap map[string]int
	// flintCount int
	ticker *time.Ticker
	db     *bolt.DB
}

func (g *Game) startFindingFlint() {
	if g.ticker == nil {
		g.ticker = time.NewTicker(1 * time.Second)
		go func() {
			for {
				<-g.ticker.C
				g.resourceMap["flint"]++
				g.saveState()
			}
		}()
	} else {
		fmt.Println("You are already finding flint.")
	}
}

// func (g *Game) stopFindingFlint() {
// 	if g.ticker != nil {
// 		g.ticker.Stop()
// 		g.ticker = nil
// 		fmt.Println("You stopped finding flint.")
// 		g.saveState()
// 	} else {
// 		fmt.Println("You are not finding flint currently.")
// 	}
// }

func (g *Game) saveState() {
	g.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))

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
		} else {
			// If there's no data in the database, initialize an empty map
			g.resourceMap = make(map[string]int)
		}
		return nil
	})
}

func main() {
	reader := bufio.NewReader(os.Stdin)

	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		fmt.Println("Failed to open the Database:", err)
		return
	}
	defer db.Close()

	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})

	game := &Game{
		db:          db,
		resourceMap: make(map[string]int),
	}
	game.loadState()

	fmt.Println("Welcome to Cividler! The CLI based idle game.")
	fmt.Println("Commands:")
	fmt.Println("\"find flint\" to start finding flint.")
	// fmt.Println("\"stop\" to stop finding flint.")
	// fmt.Println("you currently have", game.flintCount, "flint(s).")

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

		switch strings.TrimSpace(text) {
		case "find flint", "start":
			game.startFindingFlint()
		// case "stop":
		// 	game.stopFindingFlint()
		case "resources":
			fmt.Println("Resources:")
			for resource, count := range game.resourceMap {
				fmt.Printf("|%s|%d|\n", resource, count)
			}
		case "help":
			fmt.Println("Commands")
			fmt.Println("Commands:")
			fmt.Println("\"find flint\" or \"start\" to start finding flint.")
			// fmt.Println("\"stop\" to stop finding flint.")
			fmt.Println("\"resources\" to see your current resource counts.")
			fmt.Println("\"help\" to display this help message.")
		default:
			fmt.Println("Invalid command. Type \"help\" for a list of valid commands.")
		}
	}
}
