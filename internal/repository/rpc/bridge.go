/*
Package rpc implements bridge to Lachesis full node API interface.

We recommend using local IPC for fast and the most efficient inter-process communication between the API server
and an Opera/Lachesis node. Any remote RPC connection will work, but the performance may be significantly degraded
by extra networking overhead of remote RPC calls.

You should also consider security implications of opening Lachesis RPC interface for remote access.
If you considering it as your deployment strategy, you should establish encrypted channel between the API server
and Lachesis RPC interface with connection limited to specified endpoints.

We strongly discourage opening Lachesis RPC interface for unrestricted Internet access.
*/
package rpc

import (
	"context"
	"deamchain-graphql/internal/config"
	"deamchain-graphql/internal/logger"
	"deamchain-graphql/internal/repository/rpc/contracts"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	etc "github.com/ethereum/go-ethereum/core/types"
	eth "github.com/ethereum/go-ethereum/ethclient"
	deam "github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/sync/singleflight"
)

// rpcHeadProxyChannelCapacity represents the capacity of the new received blocks proxy channel.
const rpcHeadProxyChannelCapacity = 10000

// DeamBridge represents Lachesis RPC abstraction layer.
type DeamBridge struct {
	rpc *deam.Client
	eth *eth.Client
	log logger.Logger
	cg  *singleflight.Group

	// fMintCfg represents the configuration of the fMint protocol
	sigConfig     *config.ServerSignature
	sfcConfig     *config.Staking
	uniswapConfig *config.DeFiUniswap

	// extended minter config
	fMintCfg fMintConfig
	fLendCfg fLendConfig

	// common contracts
	sfcAbi      *abi.ABI
	sfcContract *contracts.SfcContract

	// received blocks proxy
	wg       *sync.WaitGroup
	sigClose chan bool
	headers  chan *etc.Header
}

// New creates new Lachesis RPC connection bridge.
func New(cfg *config.Config, log logger.Logger) (*DeamBridge, error) {
	cli, con, err := connect(cfg, log)
	if err != nil {
		log.Criticalf("can not open connection; %s", err.Error())
		return nil, err
	}

	// build the bridge structure using the con we have
	br := &DeamBridge{
		rpc: cli,
		eth: con,
		log: log,
		cg:  new(singleflight.Group),

		// special configuration options below this line
		sigConfig:     &cfg.MySignature,
		sfcConfig:     &cfg.Staking,
		uniswapConfig: &cfg.DeFi.Uniswap,
		fMintCfg: fMintConfig{
			addressProvider: cfg.DeFi.FMint.AddressProvider,
		},
		fLendCfg: fLendConfig{lendigPoolAddress: cfg.DeFi.FLend.LendingPool},

		// configure block observation loop
		wg:       new(sync.WaitGroup),
		sigClose: make(chan bool, 1),
		headers:  make(chan *etc.Header, rpcHeadProxyChannelCapacity),
	}

	// inform about the local address of the API node
	log.Noticef("using signature address %s", br.sigConfig.Address.String())

	// add the bridge ref to the fMintCfg and return the instance
	br.fMintCfg.bridge = br
	br.run()
	return br, nil
}

// connect opens connections we need to communicate with the blockchain node.
func connect(cfg *config.Config, log logger.Logger) (*deam.Client, *eth.Client, error) {
	// log what we do
	log.Debugf("connecting blockchain node at %s", cfg.Lachesis.Url)

	// try to establish a connection
	client, err := deam.Dial(cfg.Lachesis.Url)
	if err != nil {
		log.Critical(err)
		return nil, nil, err
	}

	// try to establish a for smart contract interaction
	con, err := eth.Dial(cfg.Lachesis.Url)
	if err != nil {
		log.Critical(err)
		return nil, nil, err
	}

	// log
	log.Notice("node connection open")
	return client, con, nil
}

// run starts the bridge threads required to collect blockchain data.
func (deam *DeamBridge) run() {
	deam.wg.Add(1)
	go deam.observeBlocks()
}

// terminate kills the bridge threads to end the bridge gracefully.
func (deam *DeamBridge) terminate() {
	deam.sigClose <- true
	deam.wg.Wait()
	deam.log.Noticef("rpc threads terminated")
}

// Close will finish all pending operations and terminate the Lachesis RPC connection
func (deam *DeamBridge) Close() {
	// terminate threads before we close connections
	deam.terminate()

	// do we have a connection?
	if deam.rpc != nil {
		deam.rpc.Close()
		deam.eth.Close()
		deam.log.Info("blockchain connections are closed")
	}
}

// Connection returns open Opera/Lachesis connection.
func (deam *DeamBridge) Connection() *deam.Client {
	return deam.rpc
}

// DefaultCallOpts creates a default record for call options.
func (deam *DeamBridge) DefaultCallOpts() *bind.CallOpts {
	// get the default call opts only once if called in parallel
	co, _, _ := deam.cg.Do("default-call-opts", func() (interface{}, error) {
		return &bind.CallOpts{
			Pending:     false,
			From:        deam.sigConfig.Address,
			BlockNumber: nil,
			Context:     context.Background(),
		}, nil
	})
	return co.(*bind.CallOpts)
}

// SfcContract returns instance of SFC contract for interaction.
func (deam *DeamBridge) SfcContract() *contracts.SfcContract {
	// lazy create SFC contract instance
	if nil == deam.sfcContract {
		// instantiate the contract and display its name
		var err error
		deam.sfcContract, err = contracts.NewSfcContract(deam.sfcConfig.SFCContract, deam.eth)
		if err != nil {
			deam.log.Criticalf("failed to instantiate SFC contract; %s", err.Error())
			panic(err)
		}
	}
	return deam.sfcContract
}

// SfcAbi returns a parse ABI of the AFC contract.
func (deam *DeamBridge) SfcAbi() *abi.ABI {
	if nil == deam.sfcAbi {
		ab, err := abi.JSON(strings.NewReader(contracts.SfcContractABI))
		if err != nil {
			deam.log.Criticalf("failed to parse SFC contract ABI; %s", err.Error())
			panic(err)
		}
		deam.sfcAbi = &ab
	}
	return deam.sfcAbi
}

// ObservedBlockProxy provides a channel fed with new headers observed
// by the connected blockchain node.
func (deam *DeamBridge) ObservedBlockProxy() chan *etc.Header {
	return deam.headers
}
