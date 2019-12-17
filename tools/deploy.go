package deploy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/Zilliqa/gozilliqa-sdk/account"
	"github.com/Zilliqa/gozilliqa-sdk/bech32"
	"github.com/Zilliqa/gozilliqa-sdk/contract"
	"github.com/Zilliqa/gozilliqa-sdk/keytools"
	"github.com/Zilliqa/gozilliqa-sdk/provider"
	"github.com/Zilliqa/gozilliqa-sdk/transaction"
	"github.com/Zilliqa/gozilliqa-sdk/util"
)

func testBlockchain() {
	zilliqa := provider.NewProvider("https://dev-api.zilliqa.com/")

	// These are set by the core protocol, and may vary per-chain.
	// You can manually pack the bytes according to chain id and msg version.
	// For more information: https://apidocs.zilliqa.com/?shell#getnetworkid

	const chainID = 333  // chainId of the developer testnet
	const msgVersion = 1 // current msgVersion
	VERSION := util.Pack(chainID, msgVersion)

	// Populate the wallet with an account
	const privateKey = "3375F915F3F9AE35E6B301B7670F53AD1A5BE15D8221EC7FD5E503F21D3450C8"

	user := account.NewWallet()
	user.AddByPrivateKey(privateKey)
	user.SetDefault("8254b2c9acdf181d5d6796d63320fbb20d4edd12")
	addr := keytools.GetAddressFromPrivateKey(util.DecodeHex(privateKey))
	fmt.Println("My account address is:", user.DefaultAccount.Address)
	fmt.Println("Converting from private key gives:", addr)
	bech, _ := bech32.ToBech32Address(user.DefaultAccount.Address)
	fmt.Println("The bech32 address is:", bech)

	//testing Transaction methods
	bal, _ := zilliqa.GetBalance(user.DefaultAccount.Address).Result.(map[string]interface{})["balance"]
	gas := zilliqa.GetMinimumGasPrice().Result

	fmt.Println("The balance for account ", user.DefaultAccount.Address, " is: ", bal)
	fmt.Println("The blockchain reports minimum gas price: ", gas)

	init := []contract.Value{
		{
			"_scilla_version",
			"Uint32",
			"0",
		},
		{
			"contractOwner",
			"ByStr20",
			"0x8254b2c9acdf181d5d6796d63320fbb20d4edd12",
		},

		{
			"name",
			"String",
			"ERC777",
		},
		{
			"symbol",
			"String",
			"MoonCOIN",
		},
		{
			"decimals",
			"Uint32",
			"1",
		},
		{
			"default_operators",
			"String",
			"",
		},
	}
	code, _ := ioutil.ReadFile("./FungibleToken.scilla")

	fmt.Println("Attempting to deploy Fungible Token smart contract...")

	hello := contract.Contract{
		Code:     string(code),
		Init:     init,
		Signer:   user,
		Provider: zilliqa,
	}
	nonce, err := zilliqa.GetBalance(string(user.DefaultAccount.Address)).Result.(map[string]interface{})["nonce"].(json.Number).Int64()
	if err != nil {
		fmt.Println("Nonce response error thrown: ", err)
	}
	deployParams := contract.DeployParams{
		Version:      strconv.FormatInt(int64(VERSION), 10),
		Nonce:        strconv.FormatInt(nonce+1, 10),
		GasPrice:     "10000000000",
		GasLimit:     "222560000000000",
		SenderPubKey: string(user.DefaultAccount.PublicKey),
	}
	deployTx, err := DeployWith(&hello, deployParams, "8254B2C9ACDF181D5D6796D63320FBB20D4EDD12")

	if err != nil {
		fmt.Println("Contract deployment failed with error: ", err)
	}

	deployTx.Confirm(deployTx.ID, 1000, 10, zilliqa)

	//verify that the contract is deployed

}

func DeployWith(c *contract.Contract, params contract.DeployParams, pubkey string) (*transaction.Transaction, error) {
	if c.Code == "" || c.Init == nil || len(c.Init) == 0 {
		return nil, errors.New("Cannot deploy without code or initialisation parameters.")
	}

	tx := &transaction.Transaction{
		ID:           params.ID,
		Version:      params.Version,
		Nonce:        params.Nonce,
		Amount:       "0",
		GasPrice:     params.GasPrice,
		GasLimit:     params.GasLimit,
		Signature:    "",
		Receipt:      transaction.TransactionReceipt{},
		SenderPubKey: params.SenderPubKey,
		ToAddr:       "0000000000000000000000000000000000000000",
		Code:         strings.ReplaceAll(c.Code, "/\\", ""),
		Data:         c.Init,
		Status:       0,
	}

	err2 := c.Signer.SignWith(tx, pubkey, *c.Provider)
	if err2 != nil {
		return nil, err2
	}

	rsp := c.Provider.CreateTransaction(tx.ToTransactionPayload())

	if rsp.Error != nil {
		return nil, errors.New(rsp.Error.Message)
	}

	result := rsp.Result.(map[string]interface{})
	hash := result["TranID"].(string)
	contractAddress := result["ContractAddress"].(string)

	tx.ID = hash
	tx.ContractAddress = contractAddress
	return tx, nil

}
