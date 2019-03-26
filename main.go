package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/joho/godotenv"
	"github.com/julienschmidt/httprouter"
)

// Block ... the blocks that will make up the blockchain
type Block struct {
	Index     int    // the position of the data record in the blockchain
	Timestamp string // the time the data is written
	Data      int    // the custom data, could be anything, this represents an integer
	Hash      string // SHA256 identifier representing this data record
	PrevHash  string // SHA256 identifier of the previous record in the chain
}

// Message ... to be able to take the request body of the POST req / {"Data":100}
type Message struct {
	Data int
}

// Blockchain is a slice of blocks
var Blockchain []Block

func main() {
	err := godotenv.Load() // load env file
	if err != nil {
		log.Fatal(err)
	}

	go func() { // create the genesis block in a go routine so its on a separate thread from the api
		t := time.Now()                                 // new time stamp
		genesisBlock := Block{0, t.String(), 0, "", ""} // a genesis block is the first block in a blockchain
		spew.Dump(genesisBlock)                         // log the first block
		Blockchain = append(Blockchain, genesisBlock)   // append the first block in to the blockchain
	}()

	log.Fatal(InitServer()) // run server
}

// GenerateHash creates a hash out of block data
func GenerateHash(block Block) string { // returns a string
	record := string(block.Index) + block.Timestamp + string(block.Data) + block.PrevHash // create a string of all the data
	hash := sha256.New()                                                                  // make a new hash
	hash.Write([]byte(record))
	hashed := hash.Sum(nil)
	return hex.EncodeToString(hashed) // return hexadecimal encoding of hashed string
}

// GenerateBlock returns a new block or error, based on a previous block
func GenerateBlock(prevBlock Block, Data int) (Block, error) {
	var newBlock Block                     // init block
	t := time.Now()                        // new timestamp
	newBlock.Index = prevBlock.Index + 1   // make block index prev + 1
	newBlock.Timestamp = t.String()        // set block timestamp as ts string
	newBlock.Data = Data                   // set Data as param, this is relative data (eg currency)
	newBlock.PrevHash = prevBlock.Hash     // set the previous hash as the prev blocks hash
	newBlock.Hash = GenerateHash(newBlock) // generate this blocks hash with current data

	return newBlock, nil
}

// ValidateBlock returns if a block is valid or not
func ValidateBlock(prevBlock, newBlock Block) bool {
	if prevBlock.Index+1 != newBlock.Index { // check if the previous block is actually the previous block by index
		return false
	}

	if prevBlock.Hash != newBlock.PrevHash { // check if the previous block hash matches the new block prev hash
		return false
	}

	if GenerateHash(newBlock) != newBlock.Hash { // double check the current / new block hash is valid
		return false
	}

	return true // block is valid
}

// ReplaceChain replaces the slice with the longest chain
func ReplaceChain(newBlocks []Block) {
	if len(newBlocks) > len(Blockchain) { // if the new chain is longer, replace the blockchain
		Blockchain = newBlocks
	}
}

// InitServer runs the HTTP server
func InitServer() error {
	router := MakeRouter()           // use httprouter instead of mux bcus we all about that dynamic trie structure
	HTTPAddress := os.Getenv("ADDR") // get address from env file

	log.Println("API listening on ", HTTPAddress)

	s := &http.Server{ // http config
		Addr:           ":" + HTTPAddress,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	err := s.ListenAndServe()

	if err != nil {
		return err // if the server stops working, return error
	}

	return nil // return nothing if there's no error
}

// MakeRouter creates all the http routes we'll use to view and post to our blockchain
func MakeRouter() http.Handler {
	router := httprouter.New()
	router.GET("/", GetBlockchain)
	router.POST("/", WriteBlockchain)
	return router
}

// GetBlockchain handles the route to view the blockchain
func GetBlockchain(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	bytes, err := json.MarshalIndent(Blockchain, "", " ") // marshal / parse our blockchain slice
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError) // if theres an error, freak out
		return
	}

	io.WriteString(w, string(bytes)) // write the blockchain to response
}

// WriteBlockchain handles the route to post to our blockchain
func WriteBlockchain(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var m Message // message struct for decoding the body

	decoder := json.NewDecoder(r.Body) // decode the request body
	if err := decoder.Decode(&m); err != nil {
		RespondWithJSON(w, r, http.StatusBadRequest, r.Body) // return json over http
		return
	}

	defer r.Body.Close() // close the request at the end

	newBlock, err := GenerateBlock(Blockchain[len(Blockchain)-1], m.Data) // create a new block with the POST data
	if err != nil {
		RespondWithJSON(w, r, http.StatusInternalServerError, m) // send error
		return
	}

	if ValidateBlock(Blockchain[len(Blockchain)-1], newBlock) { // validate the block
		newBlockchain := append(Blockchain, newBlock) // append the new block to blockchain
		ReplaceChain(newBlockchain)                   // replace the chain
		spew.Dump(Blockchain)                         // for logging
	}

	RespondWithJSON(w, r, http.StatusCreated, newBlock) // return json over http
}

// RespondWithJSON to handle HTTP requests
func RespondWithJSON(w http.ResponseWriter, r *http.Request, code int, payload interface{}) {
	response, err := json.MarshalIndent(payload, "", "  ") // get the json response

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("HTTP 500: Internal Server Error"))
		return
	}

	w.WriteHeader(code) // status code
	w.Write(response)   // send json over http
}
