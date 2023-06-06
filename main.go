package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

type Block struct {
	Index int
	Timestamp string
	Data int
	Hash string
	PrevHash string
	Difficulty int
	Nonce string
}

type Message struct {
	Data int
}

var Blockchain []Block

// TODO: increase difficulty over certain time
const difficulty = 1

var mutex = &sync.Mutex{}

func main()  {
	fmt.Println("Starting the go blockchain")

	// Load .env
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		// Create genesis block
		t := time.Now()
		genesisBlock := Block{}
		genesisBlock = Block{0, t.String(), 0, calculateHash(genesisBlock), "", difficulty, ""}
		// Use spew to format and dump it into console
		spew.Dump(genesisBlock)
		// Add genesis to the blockchain
		mutex.Lock()
		Blockchain = append(Blockchain, genesisBlock)
		mutex.Unlock()
	}()
	log.Fatal(run())
}

func run() error {
	// Create server
	mux := makeMuxRouter()
	httpPort := os.Getenv("PORT")
	log.Println("HTTP server is running and listening on port:", httpPort)

	s := &http.Server{
		Addr: ":" + httpPort,
		Handler: mux,
		ReadTimeout: 10*time.Second,
		WriteTimeout: 10*time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	if err := s.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

// Create router and define two routes + handlers
func makeMuxRouter() http.Handler {
	muxRouter := mux.NewRouter()
	muxRouter.HandleFunc("/", handleGetBlockchain).Methods("GET")
	muxRouter.HandleFunc("/", handleWriteBlock).Methods("POST")
	return muxRouter
}

// Handler for getting the blockchain
func handleGetBlockchain(w http.ResponseWriter, r *http.Request) {
	bytes, err := json.MarshalIndent(Blockchain, "", " ")
	if err != nil {
		log.Fatal(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, string(bytes))
}

// Handler for writing a new block
func handleWriteBlock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var m Message
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&m); err != nil {
		responseWithJSON(w, r, http.StatusBadRequest, r.Body)
		return
	}
	defer r.Body.Close()

	// Write to blockchain
	mutex.Lock()
	newBlock := generateBlock(Blockchain[len(Blockchain) - 1], m.Data)
	mutex.Unlock()
	// Check validity
	if isBlockValid(newBlock, Blockchain[len(Blockchain) - 1]) {
		// Append
		Blockchain = append(Blockchain, newBlock)
		spew.Dump(Blockchain)
	}

	// Respond with status and data
	responseWithJSON(w, r, http.StatusCreated, newBlock)
}

// Helper function
func responseWithJSON(w http.ResponseWriter, r *http.Request, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	response, err := json.MarshalIndent(payload, "", " ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("HTTP 500: Internal Server Error"))
	}
	w.WriteHeader(code)
	w.Write(response)
}

// Checks for valid next block, comparing to prev block's data
func isBlockValid(newBlock, oldBlock Block) bool {
	if oldBlock.Index + 1 != newBlock.Index {
		return false
	}
	if oldBlock.Hash != newBlock.PrevHash {
		return false
	}
	if calculateHash(newBlock) != newBlock.Hash {
		return false
	}
	return true
}

// Handling creating, hashing, and validating a new block
func generateBlock(oldBlock Block, data int) Block {
	// Create the new block data
	var newBlock Block
	t := time.Now()
	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.Data = data
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Difficulty = difficulty

	// Proof-of-work, creating hashes of the block data and validating it
	for i := 0; ; i++ {
		hex := fmt.Sprintf("/%x", i)
		newBlock.Nonce = hex
		// Hash is not valid, continue to run within the loop
		if !isHashValid(calculateHash(newBlock), newBlock.Difficulty) {
			fmt.Println(calculateHash(newBlock), "do more work!")
			time.Sleep(time.Second)
			continue
		} else {
			// Hash is valid, time to produce block and break out of the loop
			fmt.Println(calculateHash(newBlock), "work done!")
			newBlock.Hash = calculateHash(newBlock)
			break
		}
	}

	return newBlock
}

// Concat all the data, hash it with SHA256, then Encode it and return it as the hash
func calculateHash(block Block) string {
	record := strconv.Itoa(block.Index) + block.Timestamp + strconv.Itoa(block.Data) + block.PrevHash + block.Nonce
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

// Prefix with difficulty level, and check if hash has that many prefixed zeros
func isHashValid(hash string, d int) bool {
	prefix := strings.Repeat("0", d)
	return strings.HasPrefix(hash, prefix)
}