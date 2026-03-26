package deployer

type transactionReceipt struct {
	ContractAddress string `json:"contractAddress"`
	Status          string `json:"status"`
	TransactionHash string `json:"transactionHash"`
}
