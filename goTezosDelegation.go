package goTezos

/*
Author: DefinitelyNotAGoat/MagicAglet
Version: 0.0.1
Description: This file contains specific functions for delegation services
License: MIT
*/

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
)

func CalculateAllContractsForCycles(delegatedContracts []DelegatedContract, cycleStart int, cycleEnd int, rate float64, spillage bool, delegateAddr string) ([]DelegatedContract, error) {
	var err error

	for cycleStart <= cycleEnd {
		delegatedContracts, err = CalculateAllContractsForCycle(delegatedContracts, cycleStart, rate, spillage, delegateAddr)
		if err != nil {
			return delegatedContracts, errors.New("Could not calculate all commitments for cycles " + strconv.Itoa(cycleStart) + "-" + strconv.Itoa(cycleEnd) + ":CalculateAllCommitmentsForCycle(delegatedContracts []DelegatedContract, cycle int, rate float64) failed: " + err.Error())
		}
		cycleStart = cycleStart + 1
	}
	return delegatedContracts, nil
}

func CalculateAllContractsForCycle(delegatedContracts []DelegatedContract, cycle int, rate float64, spillage bool, delegateAddr string) ([]DelegatedContract, error) {
	var err error
	var balance float64
	delegationsForCycle, _ := GetDelegatedContractsForCycle(cycle, delegateAddr)

	for index, delegation := range delegatedContracts {
		balance, err = GetAccountBalanceAtSnapshot(delegation.Address, cycle)
		if err != nil {
			return delegatedContracts, errors.New("Could not calculate all commitments for cycle " + strconv.Itoa(cycle) + ":GetAccountBalanceAtSnapshot(tezosAddr string, cycle int) failed: " + err.Error())
		}
		if isDelegationInGroup(delegatedContracts[index].Address, delegationsForCycle, delegatedContracts[index].Delegate) {
			delegatedContracts[index].Contracts = append(delegatedContracts[index].Contracts, Contract{Cycle: cycle, Amount: balance})
		} else {
			delegatedContracts[index].Contracts = append(delegatedContracts[index].Contracts, Contract{Cycle: cycle, Amount: 0})
		}
	}

	delegatedContracts, err = CalculatePercentageSharesForCycle(delegatedContracts, cycle, rate, spillage, delegateAddr)
	if err != nil {
		return delegatedContracts, errors.New("func CalculateAllContractsForCycle(delegatedContracts []DelegatedContract, cycle int, rate float64, spillage bool, delegateAddr string) failed: " + err.Error())
	}
	return delegatedContracts, nil
}

func isDelegationInGroup(phk string, group []string, delegate bool) bool {
	if delegate {
		return true
	}
	for _, address := range group {
		if address == phk {
			return true
		}
	}
	return false
}

/*
Description: Calculates the share percentage of a cycle over a list of delegated contracts


*/
func CalculatePercentageSharesForCycle(delegatedContracts []DelegatedContract, cycle int, rate float64, spillage bool, delegateAddr string) ([]DelegatedContract, error) {
	var stakingBalance float64
	//var balance float64
	var err error

	spillAlert := false

	stakingBalance, err = GetDelegateStakingBalance(delegateAddr, cycle)
	if err != nil {
		return delegatedContracts, errors.New("func CalculateRollSpillage(delegatedContracts []DelegatedContract, delegateAddr string) failed: " + err.Error())
	}

	mod := math.Mod(stakingBalance, 10000)
	sum := stakingBalance - mod
	balanceCheck := stakingBalance - mod

	for index, delegation := range delegatedContracts {
		counter := 0
		for i, _ := range delegation.Contracts {
			if delegatedContracts[index].Contracts[i].Cycle == cycle {
				break
			}
			counter = counter + 1
		}
		balanceCheck = balanceCheck - delegatedContracts[index].Contracts[counter].Amount
		//fmt.Println(stakingBalance)
		if spillAlert {
			delegatedContracts[index].Contracts[counter].SharePercentage = 0
			delegatedContracts[index].Contracts[counter].RollInclusion = 0
		} else if balanceCheck < 0 && spillage {
			spillAlert = true
			delegatedContracts[index].Contracts[counter].SharePercentage = (delegatedContracts[index].Contracts[counter].Amount + stakingBalance) / sum
			delegatedContracts[index].Contracts[counter].RollInclusion = delegatedContracts[index].Contracts[counter].Amount + stakingBalance
		} else {
			delegatedContracts[index].Contracts[counter].SharePercentage = delegatedContracts[index].Contracts[counter].Amount / stakingBalance
			delegatedContracts[index].Contracts[counter].RollInclusion = delegatedContracts[index].Contracts[counter].Amount
		}
		delegatedContracts[index].Contracts[counter] = CalculatePayoutForContract(delegatedContracts[index].Contracts[counter], rate, delegatedContracts[index].Delegate, delegateAddr)
		delegatedContracts[index].Fee = delegatedContracts[index].Fee + delegatedContracts[index].Contracts[counter].Fee
	}

	return delegatedContracts, nil
}

/*
Description: Retrieves the list of addresses delegated to a delegate
Param SnapShot: A SnapShot object describing the desired snap shot.
Param delegateAddr: A string that represents a delegators tz address.
Returns []string: An array of contracts delegated to the delegator during the snap shot
*/
func GetDelegatedContractsForCycle(cycle int, delegateAddr string) ([]string, error) {
	var rtnString []string
	snapShot, err := GetSnapShot(cycle)
	// fmt.Println(snapShot)
	if err != nil {
		return rtnString, errors.New("Could not get delegated contracts for cycle " + strconv.Itoa(cycle) + ": GetSnapShot(cycle int) failed: " + err.Error())
	}
	hash, err := GetBlockHashAtLevel(snapShot.AssociatedBlock)
	if err != nil {
		return rtnString, errors.New("Could not get delegated contracts for cycle " + strconv.Itoa(cycle) + ": GetBlockLevelHash(level int) failed: " + err.Error())
	}
	// fmt.Println(hash)
	getDelegatedContracts := "/chains/main/blocks/" + hash + "/context/delegates/" + delegateAddr + "/delegated_contracts"

	s, err := TezosRPCGet(getDelegatedContracts)
	if err != nil {
		return rtnString, errors.New("Could not get delegated contracts for cycle " + strconv.Itoa(cycle) + ": TezosRPCGet(arg string) failed: " + err.Error())
	}

	DelegatedContracts, err := unMarshelStringArray(s)
	if err != nil {
		return rtnString, errors.New("Could not get delegated contracts for cycle " + strconv.Itoa(cycle) + ": You have no contracts.")
	}

	return DelegatedContracts, nil
}

/*
Description: Gets a list of all of the delegated contacts to a delegator
Param delegateAddr (string): string representation of the address of a delegator
Returns ([]string): An array of addresses (delegated contracts) that are delegated to the delegator
*/
func GetAllDelegatedContracts(delegateAddr string) ([]string, error) {
	var rtnString []string
	delegatedContractsCmd := "/chains/main/blocks/head/context/delegates/" + delegateAddr + "/delegated_contracts"
	s, err := TezosRPCGet(delegatedContractsCmd)
	if err != nil {
		return rtnString, errors.New("Could not get delegated contracts: TezosRPCGet(arg string) failed: " + err.Error())
	}

	DelegatedContracts, err := unMarshelStringArray(s)
	if err != nil {
		return rtnString, err
	}
	//fmt.Println(rtnString)
	return DelegatedContracts, nil
}

func GetDelegatedContractsBetweenContracts(cycleStart int, cycleEnd int, delegateAddr string) ([]string, error) {

	contracts, err := GetDelegatedContractsForCycle(cycleStart, delegateAddr)
	if err != nil {
		return contracts, err
	}

	cycleStart++

	for ; cycleStart <= cycleEnd; cycleStart++ {
		tmpContracts, err := GetDelegatedContractsForCycle(cycleStart, delegateAddr)
		if err != nil {
			return contracts, err
		}
		for _, tmpContract := range tmpContracts {

			found := false
			for _, mainContract := range contracts {
				if mainContract == tmpContract {
					found = true
				}
			}
			if !found {
				contracts = append(contracts, tmpContract)
			}
		}
	}
	return contracts, nil
}

/*
Description: Takes a commitment, and calculates the GrossPayout, NetPayout, and Fee.
Param commitment (Commitment): The commitment we are doing the operation on.
Param rate (float64): The delegation percentage fee written as decimal.
Param totalNodeRewards: Total rewards for the cyle the commitment represents. //TODO Make function to get total rewards for delegate in cycle
Param delegate (bool): Is this the delegate
Returns (Commitment): Returns a commitment with the calculations made
Note: This function assumes Commitment.SharePercentage is already calculated.
*/
func CalculatePayoutForContract(contract Contract, rate float64, delegate bool, delegateAddr string) Contract {

	totalNodeRewards, _ := GetDelegateRewardsForCycle(contract.Cycle, delegateAddr)

	grossRewards := contract.SharePercentage * float64(totalNodeRewards)
	contract.GrossPayout = grossRewards
	fee := rate * grossRewards
	contract.Fee = fee
	var netRewards float64
	if delegate {
		netRewards = grossRewards
		contract.NetPayout = netRewards
		contract.Fee = 0
	} else {
		netRewards = grossRewards - fee
		contract.NetPayout = contract.NetPayout + netRewards
	}

	return contract
}

func GetDelegateRewardsForCycle(cycle int, delegate string) (float64, error) {
	var rtn float64
	rtn = 0
	get := "/chains/main/blocks/head/context/raw/json/contracts/index/" + delegate + "/frozen_balance/" + strconv.Itoa(cycle) + "/"
	s, err := TezosRPCGet(get)
	if err != nil {
		return rtn, errors.New("Could not get rewards for delegate: " + err.Error())
	}
	rewards, err := unMarshelFrozenBalanceRewards(s)
	if err != nil {
		return rtn, errors.New("Could not get rewards for delegate, could not parse.")
	}

	iRewards, err := strconv.Atoi(rewards.Rewards)
	if err != nil {
		return rtn, errors.New("Could not get rewards for delegate: " + err.Error())
	}

	rtn = float64(iRewards) / 1000000

	return rtn, nil
}

/*
Description: Calculates all the fees for each contract and adds them to the delegates net payout


*/
func CalculateDelegateNetPayout(delegatedContracts []DelegatedContract) []DelegatedContract {
	var delegateIndex int

	for index, delegate := range delegatedContracts {
		if delegate.Delegate {
			delegateIndex = index
		}
	}

	for _, delegate := range delegatedContracts {
		if !delegate.Delegate {
			delegatedContracts[delegateIndex].TotalPayout = delegatedContracts[delegateIndex].TotalPayout + delegate.Fee
		}
	}
	return delegatedContracts
}

/*
Description: A function to Payout rewards for all contracts in delegatedContracts
Param delegatedContracts ([]DelegatedClient): List of all contracts to be paid out
Param alias (string): The alias name to your known delegation wallet on your node
****WARNING****
If not using the ledger there is nothing stopping this from actually sending Tezos.
With the ledger you have to physically confirm the transaction, without the ledger you don't.
BE CAREFUL WHEN CALLING THIS FUNCTION!!!!!
****WARNING****
*/
// func PayoutDelegatedContracts(delegatedContracts []DelegatedContract, alias string) error {
// 	for _, delegatedContract := range delegatedContracts {
// 		err := SendTezos(delegatedContract.TotalPayout, delegatedContract.Address, alias)
// 		if err != nil {
// 			return errors.New("Could not Payout Delegated Contracts: SendTezos(amount float64, toAddress string, alias string) failed: " + err.Error())
// 		}
// 	}
// 	return nil
// }

/*
Description: Calculates the total payout in all commitments for a delegated contract
Param delegatedContracts (DelegatedClient): the delegated contract to calulate over
Returns (DelegatedClient): return the contract with the Total Payout
*/
func CalculateTotalPayout(delegatedContract DelegatedContract) DelegatedContract {
	for _, contract := range delegatedContract.Contracts {
		delegatedContract.TotalPayout = delegatedContract.TotalPayout + contract.NetPayout
	}
	return delegatedContract
}

/*
Description: payout in all commitments for a delegated contract for all contracts
Param delegatedContracts (DelegatedClient): the delegated contracts to calulate over
Returns (DelegatedClient): return the contract with the Total Payout for all contracts
*/
func CalculateAllTotalPayout(delegatedContracts []DelegatedContract) []DelegatedContract {
	for index, delegatedContract := range delegatedContracts {
		delegatedContracts[index] = CalculateTotalPayout(delegatedContract)
	}

	return delegatedContracts
}

/*
Description: A test function that loops through the commitments of each delegated contract for a specific cycle,
             then it computes the share value of each one. The output should be = 1. With my tests it was, so you
             can really just ignore this.
Param cycle (int): The cycle number to be queryed
Param delegatedContracts ([]DelegatedClient): the group of delegated DelegatedContracts
Returns (float64): The sum of all shares
*/
func CheckPercentageSumForCycle(cycle int, delegatedContracts []DelegatedContract) float64 {
	var sum float64
	sum = 0
	for x := 0; x < len(delegatedContracts); x++ {
		counter := 0
		for y := 0; y < len(delegatedContracts[x].Contracts); y++ {
			if delegatedContracts[x].Contracts[y].Cycle == cycle {
				break
			}
			counter = counter + 1
		}

		sum = sum + delegatedContracts[x].Contracts[counter].SharePercentage
	}
	return sum
}

/*
Description: A function to account for incomplete rolls, and the payouts associated with that
TODO: In Progress
*/
func CalculateRollSpillage(delegatedContracts []DelegatedContract, delegateAddr string, cycle int) ([]DelegatedContract, error) {
	stakingBalance, err := GetDelegateStakingBalance(delegateAddr, cycle)
	if err != nil {
		return delegatedContracts, errors.New("func CalculateRollSpillage(delegatedContracts []DelegatedContract, delegateAddr string) failed: " + err.Error())
	}

	mod := math.Mod(stakingBalance, 10000)
	sum := mod * 10000

	for index, delegatedContract := range delegatedContracts {
		for i, contract := range delegatedContract.Contracts {
			if contract.Cycle == cycle {
				stakingBalance = stakingBalance - contract.Amount
				if stakingBalance < 0 {
					delegatedContracts[index].Contracts[i].SharePercentage = (contract.Amount - stakingBalance) / sum
				}
			}
		}
	}

	return delegatedContracts, nil
}

/*
Description: Reverse the order of an array of DelegatedClient.
             Used when fisrt retreiving contracts because the
             Tezos RPC API returns the newest contract first.
Param delegatedContracts ([]DelegatedClient) Delegated
*/
func SortDelegateContracts(delegatedContracts []DelegatedContract) []DelegatedContract {
	for i, j := 0, len(delegatedContracts)-1; i < j; i, j = i+1, j-1 {
		delegatedContracts[i], delegatedContracts[j] = delegatedContracts[j], delegatedContracts[i]
	}
	return delegatedContracts
}

type TransOp struct {
	Kind         string `json:"kind"`
	Amount       string `json:"amount"`
	Source       string `json:"source"`
	Destination  string `json:"destination"`
	StorageLimit string `json:"storage_limit"`
	GasLimit     string `json:"gas_limit"`
	Fee          string `json:"fee"`
	Counter      string `json:"counter"`
}

type Conts struct {
	Contents []TransOp `json:"contents"`
	Branch   string    `json:"branch"`
}

func PayoutContracts(delegatedContracts []DelegatedContract, source string) error {
	var contents Conts
	var transOps []TransOp
	for _, contract := range delegatedContracts {
		if contract.Address != source {
			pay := strconv.FormatFloat(contract.TotalPayout, 'f', 6, 64)
			i, err := strconv.ParseFloat(pay, 64)
			if err != nil {
				return errors.New("Could not get parse amount to payout: " + err.Error())
			}
			i = i * 1000000
			if i != 0 {
				transOps = append(transOps, TransOp{Kind: "transaction", Source: source, Fee: "1", GasLimit: "100", StorageLimit: "0", Amount: strconv.Itoa(int(i)), Destination: contract.Address})
			}

		}
	}
	contents.Contents = transOps
	var err error
	_, contents.Branch, err = GetBlockLevelHead()
	if err != nil {
		return errors.New("Could not get head hash: " + err.Error())
	}

	post, err := json.Marshal(contents)
	if err != nil {
		return errors.New("Could not marshel trans object: " + err.Error())
	}

	//req := "/chains/main/blocks/head/helpers/forge/operations"

	// err = TezosRPCPost(req, post)
	// if err != nil {
	// 	return errors.New("Could not post transactions: " + err.Error())
	// }

	fmt.Println(PrettyReport(string(post)))
	return nil
}

// {
//     "contents": [
//         {
//             "kind": "transaction",
//             "amount": "1",
//             "source": "tz1ABCDEF",
//             "destination": "tz1GHIJKL",
//             "storage_limit": "0",
//             "gas_limit": "0",
//             "fee": "0",
//             "counter": "0"
//         }
//     ],
//     "branch": "BM6fGF1GBDkHaYmBpw613izL9YhpBSSGVaJVEvibYfwVoCtzHLn"
// }

// { "kind": "transaction",
//          "source": $contract_id,
//          "fee": $mutez,
//          "counter": $positive_bignum,
//          "gas_limit": $positive_bignum,
//          "storage_limit": $positive_bignum,
//          "amount": $mutez,
//          "destination": $contract_id,
//          "parameters"?: $micheline.michelson_v1.expression }