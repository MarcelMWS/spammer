package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
)

/*
	usage
	- spammer fatTx <number>
	- gaiad tx sign fatTx.json --chain-id sc --keyring-backend test --keyring-dir . --from main --node tcp://35.157.124.89:26657 > signed.json
	- gaiad keys list --keyring-backend test --keyring-dir . --output json > addrs.json
	- spammer bulkTxs
*/

func main() {
	flag.Parse()
	action := flag.Arg(0)

	switch action {
	case "fatTx":
		generateAccountsAndFatTX()
	case "bulkTxs":
		createBulkTxs()
	default:
		fmt.Println("Invalid command. Use 'fatTx [amount of accounts]' or 'bulkTxs'")
	}
}

func generateAccountsAndFatTX() {
	numberString := flag.Arg(1)
	amount, err := strconv.Atoi(numberString)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	if amount < 1 {
		fmt.Println("amount must be > 0")
		os.Exit(1)
	}

	fmt.Printf("Generating %d accounts... \n", amount)

	addresses := make([]string, amount)

	for i := 1; i <= amount; i++ {
		ai := generateAccountCMD(i)
		addresses[i-1] = ai.Address
	}

	buildFundTx(addresses)
}

func createBulkTxs() {
	// Load addresses
	fmt.Println("Loading adresses from 'addrs.json'")
	bz, err := ioutil.ReadFile("addrs.json")
	if err != nil {
		fmt.Println("couldn't read file addrs.json")
		os.Exit(1)
	}

	addrs := []accountInfo{}
	err = json.Unmarshal(bz, &addrs)
	if err != nil {
		fmt.Println("Couldn't unmarshal addresses", err.Error())
		os.Exit(1)
	}

	// generate signed transactions
	fmt.Println("Creating directories")
	os.MkdirAll("txs/unsigned", os.ModePerm)
	os.MkdirAll("txs/signed", os.ModePerm)

	fmt.Println("generating signed transactions for all accounts")

	for _, addr := range addrs {
		buildSendTx(addr.Address)
	}

	for _, addr := range addrs {
		broadCastSendTx(addr.Address)
	}

}

// Fund tx builder

type BasicTx struct {
	Body       Body          `json:"body"`
	AuthInfo   AuthInfo      `json:"auth_info"`
	Signatures []interface{} `json:"signatures"`
}

type Body struct {
	Messages                    []BasicSendMsg `json:"messages"`
	Memo                        string         `json:"memo"`
	TimeoutHeight               string         `json:"timeout_height"`
	ExtensionOptions            []interface{}  `json:"extension_options"`
	NonCriticalExtensionOptions []interface{}  `json:"non_critical_extension_options"`
}
type AuthInfo struct {
	SignerInfos []interface{} `json:"signer_infos"`
	Fee         Fee           `json:"fee"`
}

type Fee struct {
	Amount   []Amount `json:"amount"`
	GasLimit string   `json:"gas_limit"`
	Payer    string   `json:"payer"`
	Granter  string   `json:"granter"`
}

type BasicSendMsg struct {
	Type        string   `json:"@type"`
	FromAddress string   `json:"from_address"`
	ToAddress   string   `json:"to_address"`
	Amount      []Amount `json:"amount"`
}

type Amount struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}

func buildFundTx(addresses []string) {
	// generate massive tx with send msg for every account

	msgs := make([]BasicSendMsg, len(addresses))

	for i, addr := range addresses {
		msgs[i] = BasicSendMsg{
			Type:        "/cosmos.bank.v1beta1.MsgSend",
			FromAddress: "cosmos1wjdgeersnvwf9e7w7j54v7lu9yflvwe68smq0h",
			ToAddress:   addr,
			Amount: []Amount{Amount{
				Denom:  "umuon",
				Amount: "1",
			}},
		}

	}

	tx := BasicTx{
		Body: Body{
			Messages:                    msgs,
			Memo:                        "blockscape",
			TimeoutHeight:               "0",
			ExtensionOptions:            []interface{}{},
			NonCriticalExtensionOptions: []interface{}{},
		},
		AuthInfo: AuthInfo{
			SignerInfos: []interface{}{},
			Fee: Fee{
				Amount: []Amount{}, /* []Amount{Amount{
					/* Denom:  ,
					Amount:
				}} */
				GasLimit: "200000000",
				Payer:    "",
				Granter:  "",
			},
		},
		Signatures: []interface{}{},
	}

	bz, err := json.Marshal(tx)
	if err != nil {
		fmt.Println("Couldn't marshal tx", err.Error())
		os.Exit(1)
	}

	err = ioutil.WriteFile("fatTx.json", bz, 0644)
	if err != nil {
		fmt.Println("Couldn't write fat tx file", err.Error())
		os.Exit(1)
	}

}

func buildSendTx(from string) {
	tx := BasicTx{
		Body: Body{
			Messages: []BasicSendMsg{
				BasicSendMsg{
					Type:        "/cosmos.bank.v1beta1.MsgSend",
					FromAddress: from,
					ToAddress:   "cosmos1wjdgeersnvwf9e7w7j54v7lu9yflvwe68smq0h",
					Amount: []Amount{Amount{
						Denom:  "umuon",
						Amount: "1",
					}},
				}},
			Memo:                        "blockscape",
			TimeoutHeight:               "0",
			ExtensionOptions:            []interface{}{},
			NonCriticalExtensionOptions: []interface{}{},
		},
		AuthInfo: AuthInfo{
			SignerInfos: []interface{}{},
			Fee: Fee{
				Amount:   []Amount{},
				GasLimit: "200000",
				Payer:    "",
				Granter:  "",
			},
		},
		Signatures: []interface{}{},
	}

	bz, err := json.Marshal(tx)
	if err != nil {
		fmt.Println("Couldn't marshal tx", err.Error())
		os.Exit(1)
	}

	filenameUnsigned := fmt.Sprintf("txs/unsigned/%s.json", from)

	err = ioutil.WriteFile(filenameUnsigned, bz, 0644)
	if err != nil {
		fmt.Println("Couldn't write", filenameUnsigned, ":", err.Error())
		os.Exit(1)
	}

	//sign
	signTxCmd(from)
}

// Account generation

type accountInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Address  string `json:"address"`
	Pubkey   string `json:"pubkey"`
	Mnemonic string `json:"mnemonic"`
}

func generateAccountCMD(accountNumber int) accountInfo {
	fmt.Println("Generating Account", accountNumber)
	accCmd := exec.Command("gaiad", "keys", "add", strconv.Itoa(accountNumber), "--keyring-backend", "test", "--output", "json", "--keyring-dir", ".")
	bz, err := accCmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(bz))
		fmt.Println(err.Error())
		os.Exit(1)
	}
	ai := accountInfo{}
	err = json.Unmarshal(bz, &ai)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	return ai
}

func signTxCmd(address string) {
	fmt.Println("signing tx of account", address)
	accCmd := exec.Command("gaiad", "tx", "sign", fmt.Sprintf("txs/unsigned/%s.json", address),
		"--from", address,
		"--chain-id", "sc",
		"--keyring-backend", "test",
		"--keyring-dir", ".",
		"--node", "tcp://52.59.242.1:26657")

	bz, err := accCmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(bz))
		fmt.Println(err.Error())
		os.Exit(1)
	}

	filenameSigned := "txs/signed/" + address
	err = ioutil.WriteFile(filenameSigned, bz, 0644)
	if err != nil {
		fmt.Println("Couldn't write", filenameSigned, ":", err.Error())
		os.Exit(1)
	}

}

func broadCastSendTx(address string) {
	fmt.Println("broadcasting", address)
	bcCmd := exec.Command("gaiad", "tx", "broadcast", fmt.Sprintf("txs/signed/%s", address),
		"--node", "tcp://52.59.242.1:26657", "-b", "async")

	bz2, err := bcCmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(bz2))
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
