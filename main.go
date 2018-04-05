package main


import (
	"log"
	"net"
	"crypto/sha256"
	"time"
	"encoding/hex"
	"os"
	"io"
	"bufio"
	"sync"
	"math/rand"

	"github.com/joho/godotenv"
	"github.com/davecgh/go-spew/spew"

	"strconv"
	"fmt"
)

type Block struct {
	Index int
	Timestamp string
	Data string
	PrevHash string
	Hash string
	Validator string
}


var Blockchain = []Block{}
var Candidates = []Block{}
var Validators = make(map[string]int)

var BlockMutex = &sync.Mutex{}
var ValidatorsMutex = &sync.Mutex{}
var CandidatesMutex = &sync.Mutex{}



func main(){
	log.Println("Simple POS!!!!")

	// Load configuration file
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}


	// Setup server
	httpPort := os.Getenv("PORT")
	if httpPort == "" {
		httpPort = "9000"
	}
	server, err := net.Listen("tcp", ":" + httpPort)
	if err != nil {
		log.Fatal("Cannot start Server at PORT", httpPort, err)
	}
	log.Println("STARTING SERVER AT", httpPort)
	defer server.Close()

	genesisBlock := generateBlock(0, "", "", "")
	log.Println("GenesisBlock:", spew.Sdump(genesisBlock))
	Blockchain = append(Blockchain, genesisBlock)

	var submitC = make(chan Block)
	var broadcastC = make(chan string)

	// Receiving Candidate Blocks
	go func(submitC <-chan Block){
		for {
			newBlock := <- submitC
			log.Println("Received a block candidate", spew.Sdump(newBlock))

			// getting previous Block
			BlockMutex.Lock()
			lastBlock := Blockchain[len(Blockchain) - 1]
			BlockMutex.Unlock()

			if isValidBlock(newBlock, lastBlock){
				CandidatesMutex.Lock()
				Candidates = append(Candidates, newBlock)
				CandidatesMutex.Unlock()
			} else {
				log.Fatal("Invalid block")
			}
		}
	}(submitC)


	// Block Selection

	go func(){
		for {
			selectBlock(broadcastC)
		}
	}()

	for {
		conn, err := server.Accept()
		if err != nil {
			log.Fatal("Error in accept Client connection", err)
		}
		go clientHandle(conn, submitC, broadcastC)
	}
}

//*****************************************************
//				BLOCK SELECTION
//*****************************************************
func selectBlock(broadcastC chan<- string){
	time.Sleep(10 * time.Second)
	CandidatesMutex.Lock()
	snapshot := Candidates
	Candidates = []Block{}
	CandidatesMutex.Unlock()

	log.Println("Number of block candidate: ", len(snapshot))
	if len(snapshot) < 1 {
		return
	}


	// gather a pool of block with the number of replicas is the stake/balance of its validator
	currentValidators := make(map[string]bool)
	pool := []*Block{}
	for _, block := range snapshot {
		validator := block.Validator
		if currentValidators[validator] == false {
			currentValidators[validator] = true


			stake := Validators[validator]
			for i:=0; i < stake; i++{
				pool = append(pool, &block)
			}
		}
	}

	// Random selection
	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)
	number := r.Intn(len(pool))
	log.Println("Random number: ", number, "/", len(pool))
	selected := pool[number]

	// Add to Blockchain
	log.Println("Selected block: ", spew.Sdump(*selected))
	BlockMutex.Lock()
	Blockchain = append(Blockchain, *selected)
	tempBlockchain := Blockchain
	BlockMutex.Unlock()

	//Broadcast
	for _ = range Validators{
		broadcastC <- fmt.Sprintf("NEW BLOCK: %s from Validator: %s\n BLOCKCHAIN: %s\n",
			spew.Sdump(*selected), selected.Validator, spew.Sdump(tempBlockchain))
	}



}


//*****************************************************
//					CLIENT HANDLER
//*****************************************************

func clientHandle(conn net.Conn, submitC chan<- Block, broadcastC <-chan string ) {
	defer conn.Close()


	// Get client info
	io.WriteString(conn, "Enter your name: ")
	scanner := bufio.NewScanner(conn)
	var clientName string
	var balance int
	if scanner.Scan() {
		clientName = scanner.Text()

	}
	io.WriteString(conn, "Enter your balance:")
	if scanner.Scan(){
		balance, _ = strconv.Atoi(scanner.Text())
	}

	log.Println("New client: ", clientName, "Balance: ", balance)
	ValidatorsMutex.Lock()
	Validators[clientName] = balance
	ValidatorsMutex.Unlock()



	io.WriteString(conn, "Enter your data:")
	// Start making a block
	go func() {
		for scanner.Scan() {
			data := scanner.Text()

			if data == "" {
				log.Println("EXIT!.....")
			}

			BlockMutex.Lock()
			lastBlock := Blockchain[len(Blockchain) - 1]
			BlockMutex.Unlock()

			newBlock := generateBlock(lastBlock.Index + 1,
									 data,
									 lastBlock.Hash,
									 clientName)

			submitC <- newBlock
			io.WriteString(conn, "Enter your data:")
		}
	}()

	for {
		message := <- broadcastC
		io.WriteString(conn, message)
	}
}



//*****************************************************
//					UTILITIES
//*****************************************************

func hash(input string) string {
	h := sha256.New()
	h.Write([]byte(input))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}


func hashBlock(input Block) string {
	plainText := string(input.Index) + input.Timestamp + input.Data + input.PrevHash
	return hash(plainText)
}

func isValidBlock(block Block, lastBlock Block) bool {
	if ( block.Hash != hashBlock(block)) {
		log.Fatal("Incorrect hash for block")
	}
	if ( block.Index != lastBlock.Index + 1) {
		log.Fatal("Incorrect Block Index")
	}
	if ( block.PrevHash != lastBlock.Hash){
		log.Fatal("Incorrect previous Block Hash")
	}
	return block.Hash == hashBlock(block) && block.Index == lastBlock.Index + 1 && block.PrevHash == lastBlock.Hash
}

func generateBlock(index int, data string, prevHash string, validator string) Block {
	block := Block{
		Index: index,
		Timestamp: time.Now().String(),
		Data: data,
		PrevHash: prevHash,
		Hash: "",
		Validator: validator,
	}
	block.Hash = hashBlock(block)
	return block
}

