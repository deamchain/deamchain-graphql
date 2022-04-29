/*
Package rpc implements bridge to Lachesis full node API interface.

We recommend using local IPC for fast and the most efficient inter-process communication between the API server
and an Opera/Lachesis node. Any remote RPC connection will work, but the performance may be significantly degraded
by extra networking overhead of remote RPC calls.

You should also consider security implications of opening Lachesis RPC interface for a remote access.
If you considering it as your deployment strategy, you should establish encrypted channel between the API server
and Lachesis RPC interface with connection limited to specified endpoints.

We strongly discourage opening Lachesis RPC interface for unrestricted Internet access.
*/
package rpc

//go:generate tools/abigen.sh --abi ./contracts/abi/sfc-1.1.abi --pkg contracts --type SfcV1Contract --out ./contracts/sfc-v1.go
//go:generate tools/abigen.sh --abi ./contracts/abi/sfc-2.0.4-rc.2.abi --pkg contracts --type SfcV2Contract --out ./contracts/sfc-v2.go
//go:generate tools/abigen.sh --abi ./contracts/abi/sfc-3.0-rc.1.abi --pkg contracts --type SfcContract --out ./contracts/sfc-v3.go
//go:generate tools/abigen.sh --abi ./contracts/abi/sfc-tokenizer.abi --pkg contracts --type SfcTokenizer --out ./contracts/sfc_tokenizer.go

import (
	"deamchain-graphql/internal/types"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

// sfcFirstLockEpoch represents the first epoch with stake locking available.
const sfcFirstLockEpoch uint64 = 1600

// SfcVersion returns current version of the SFC contract as a single number.
func (deam *DeamBridge) SfcVersion() (hexutil.Uint64, error) {
	// get the version information from the contract
	var ver [3]byte
	var err error
	ver, err = deam.SfcContract().Version(nil)
	if err != nil {
		deam.log.Criticalf("failed to get the SFC version; %s", err.Error())
		return 0, err
	}
	return hexutil.Uint64((uint64(ver[0]) << 16) | (uint64(ver[1]) << 8) | uint64(ver[2])), nil
}

// CurrentEpoch extract the current epoch id from SFC smart contract.
func (deam *DeamBridge) CurrentEpoch() (hexutil.Uint64, error) {
	// get the value from the contract
	epoch, err := deam.SfcContract().CurrentEpoch(deam.DefaultCallOpts())
	if err != nil {
		deam.log.Errorf("failed to get the current sealed epoch: %s", err.Error())
		return 0, err
	}
	return hexutil.Uint64(epoch.Uint64()), nil
}

// CurrentSealedEpoch extract the current sealed epoch id from SFC smart contract.
func (deam *DeamBridge) CurrentSealedEpoch() (hexutil.Uint64, error) {
	// get the value from the contract
	epoch, err := deam.SfcContract().CurrentSealedEpoch(deam.DefaultCallOpts())
	if err != nil {
		deam.log.Errorf("failed to get the current sealed epoch: %s", err.Error())
		return 0, err
	}
	return hexutil.Uint64(epoch.Uint64()), nil
}

// Epoch extract information about an epoch from SFC smart contract.
func (deam *DeamBridge) Epoch(id hexutil.Uint64) (*types.Epoch, error) {
	// extract epoch snapshot
	epo, err := deam.SfcContract().GetEpochSnapshot(nil, big.NewInt(int64(id)))
	if err != nil {
		deam.log.Errorf("failed to extract epoch information: %s", err.Error())
		return nil, err
	}

	return &types.Epoch{
		Id:                    id,
		EndTime:               hexutil.Uint64(epo.EndTime.Uint64()),
		EpochFee:              (hexutil.Big)(*epo.EpochFee),
		TotalBaseRewardWeight: (hexutil.Big)(*epo.TotalBaseRewardWeight),
		TotalTxRewardWeight:   (hexutil.Big)(*epo.TotalTxRewardWeight),
		BaseRewardPerSecond:   (hexutil.Big)(*epo.BaseRewardPerSecond),
		StakeTotalAmount:      (hexutil.Big)(*epo.TotalStake),
		TotalSupply:           (hexutil.Big)(*epo.TotalSupply),
	}, nil
}

// RewardsAllowed returns if the rewards can be manipulated with.
func (deam *DeamBridge) RewardsAllowed() (bool, error) {
	deam.log.Debug("rewards lock always open")
	return true, nil
}

// LockingAllowed indicates if the stake locking has been enabled in SFC.
func (deam *DeamBridge) LockingAllowed() (bool, error) {
	// get the current sealed epoch value from the contract
	epoch, err := deam.SfcContract().CurrentSealedEpoch(nil)
	if err != nil {
		deam.log.Errorf("failed to get the current sealed epoch: %s", err.Error())
		return false, err
	}

	return epoch.Uint64() >= sfcFirstLockEpoch, nil
}

// TotalStaked returns the total amount of staked tokens.
func (deam *DeamBridge) TotalStaked() (*big.Int, error) {
	return deam.SfcContract().TotalStake(deam.DefaultCallOpts())
}

// SfcMinValidatorStake extracts a value of minimal validator self stake.
func (deam *DeamBridge) SfcMinValidatorStake() (*big.Int, error) {
	return deam.SfcContract().MinSelfStake(deam.DefaultCallOpts())
}

// SfcMaxDelegatedRatio extracts a ratio between self delegation and received stake.
func (deam *DeamBridge) SfcMaxDelegatedRatio() (*big.Int, error) {
	return deam.SfcContract().MaxDelegatedRatio(deam.DefaultCallOpts())
}

// SfcMinLockupDuration extracts a minimal lockup duration.
func (deam *DeamBridge) SfcMinLockupDuration() (*big.Int, error) {
	return deam.SfcContract().MinLockupDuration(deam.DefaultCallOpts())
}

// SfcMaxLockupDuration extracts a maximal lockup duration.
func (deam *DeamBridge) SfcMaxLockupDuration() (*big.Int, error) {
	return deam.SfcContract().MaxLockupDuration(deam.DefaultCallOpts())
}

// SfcWithdrawalPeriodEpochs extracts a minimal number of epochs between un-delegate and withdraw.
func (deam *DeamBridge) SfcWithdrawalPeriodEpochs() (*big.Int, error) {
	return deam.SfcContract().WithdrawalPeriodEpochs(deam.DefaultCallOpts())
}

// SfcWithdrawalPeriodTime extracts a minimal number of seconds between un-delegate and withdraw.
func (deam *DeamBridge) SfcWithdrawalPeriodTime() (*big.Int, error) {
	return deam.SfcContract().WithdrawalPeriodTime(deam.DefaultCallOpts())
}
