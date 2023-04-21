package main

import (
	"context"
	"fmt"
	"os"

	miner2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/miner"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/builtin"
	lotusapi "github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/lotus/chain/types"
	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
)

var log = logging.Logger("mineraddrctl")

func main() {
	local := []*cli.Command{
		ownerCmd,
		workerCmd,
		controlCmd,
	}

	app := &cli.App{
		Name:     "mineraddrctl",
		Commands: local,
		Usage:    "manage miner addresses",
	}
	app.Setup()

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}

}

var ownerCmd = &cli.Command{
	Name: "owner",
	Subcommands: []*cli.Command{
		transferCmd,
	},
}

var transferCmd = &cli.Command{
	Name:  "transfer",
	Usage: "newAddr senderAddr",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name: "miner",
		},
		&cli.BoolFlag{
			Name:  "wait",
			Usage: "wait for the transaction to be confirmed",
			Value: false,
		},
		&cli.BoolFlag{
			Name:  "really-do-it",
			Usage: "Actually send transaction performing the action",
			Value: false,
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := context.Background()

		api, acloser, err := LotusClient(ctx)
		if err != nil {
			return err
		}
		defer acloser()

		na, err := address.NewFromString(cctx.Args().First())
		if err != nil {
			return err
		}

		newAddrId, err := api.StateLookupID(ctx, na, types.EmptyTSK)
		if err != nil {
			return err
		}

		fa, err := address.NewFromString(cctx.Args().Get(1))
		if err != nil {
			return err
		}

		fromAddrId, err := api.StateLookupID(ctx, fa, types.EmptyTSK)
		if err != nil {
			return err
		}

		maddr, err := address.NewFromString(cctx.String("miner"))
		if err != nil {
			return err
		}

		mi, err := api.StateMinerInfo(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		if fromAddrId != mi.Owner && fromAddrId != newAddrId {
			return fmt.Errorf("from address must either be the old owner or the new owner")
		}

		if !cctx.Bool("really-do-it") {
			fmt.Fprintln(cctx.App.Writer, "Pass --really-do-it to actually execute this action")
			return nil
		}

		sp, err := actors.SerializeParams(&newAddrId)
		if err != nil {
			return fmt.Errorf("serializing params: %w", err)
		}

		smsg, err := api.MpoolPushMessage(ctx, &types.Message{
			From:   fromAddrId,
			To:     maddr,
			Method: builtin.MethodsMiner.ChangeOwnerAddress,
			Value:  big.Zero(),
			Params: sp,
		}, nil)
		if err != nil {
			return fmt.Errorf("mpool push: %w", err)
		}

		fmt.Println("Message CID:", smsg.Cid())

		if cctx.Bool("wait") {
			// wait for it to get mined into a block
			wait, err := api.StateWaitMsg(ctx, smsg.Cid(), build.MessageConfidence, lotusapi.LookbackNoLimit, true)
			if err != nil {
				return err
			}

			// check it executed successfully
			if wait.Receipt.ExitCode != 0 {
				fmt.Println("owner change failed!")
				return err
			}

			fmt.Println("message succeeded!")
		}

		return nil
	},
}

var workerCmd = &cli.Command{
	Name: "worker",
	Subcommands: []*cli.Command{
		actorProposeChangeWorker,
		actorConfirmChangeWorker,
	},
}

var actorProposeChangeWorker = &cli.Command{
	Name:      "propose-change",
	Usage:     "Propose a worker address change",
	ArgsUsage: "[address]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name: "miner",
		},
		&cli.BoolFlag{
			Name:  "really-do-it",
			Usage: "Actually send transaction performing the action",
			Value: false,
		},
		&cli.BoolFlag{
			Name:  "q",
			Usage: "minimize output",
		},
	},
	Action: func(cctx *cli.Context) error {
		if !cctx.Args().Present() {
			return fmt.Errorf("must pass address of new worker address")
		}
		ctx := context.Background()

		api, acloser, err := LotusClient(ctx)
		if err != nil {
			return err
		}
		defer acloser()

		na, err := address.NewFromString(cctx.Args().First())
		if err != nil {
			return err
		}

		newAddr, err := api.StateLookupID(ctx, na, types.EmptyTSK)
		if err != nil {
			return err
		}

		maddr, err := address.NewFromString(cctx.String("miner"))
		if err != nil {
			return err
		}

		mi, err := api.StateMinerInfo(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		if mi.NewWorker.Empty() {
			if mi.Worker == newAddr {
				return fmt.Errorf("worker address already set to %s", na)
			}
		} else {
			if mi.NewWorker == newAddr {
				return fmt.Errorf("change to worker address %s already pending", na)
			}
		}

		if !cctx.Bool("really-do-it") {
			fmt.Fprintln(cctx.App.Writer, "Pass --really-do-it to actually execute this action")
			return nil
		}

		cwp := &miner2.ChangeWorkerAddressParams{
			NewWorker:       newAddr,
			NewControlAddrs: mi.ControlAddresses,
		}

		sp, err := actors.SerializeParams(cwp)
		if err != nil {
			return fmt.Errorf("serializing params: %w", err)
		}

		smsg, err := api.MpoolPushMessage(ctx, &types.Message{
			From:   mi.Owner,
			To:     maddr,
			Method: builtin.MethodsMiner.ChangeWorkerAddress,
			Value:  big.Zero(),
			Params: sp,
		}, nil)
		if err != nil {
			return fmt.Errorf("mpool push: %w", err)
		}

		fmt.Fprintln(cctx.App.Writer, "Propose Message CID:", smsg.Cid())

		// wait for it to get mined into a block
		wait, err := api.StateWaitMsg(ctx, smsg.Cid(), build.MessageConfidence, lotusapi.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		// check it executed successfully
		if wait.Receipt.ExitCode != 0 {
			fmt.Fprintln(cctx.App.Writer, "Propose worker change failed!")
			return err
		}

		mi, err = api.StateMinerInfo(ctx, maddr, wait.TipSet)
		if err != nil {
			return err
		}
		if mi.NewWorker != newAddr {
			return fmt.Errorf("proposed worker address change not reflected on chain: expected '%s', found '%s'", na, mi.NewWorker)
		}

		if !cctx.Bool("q") {
			fmt.Fprint(cctx.App.Writer, mi.WorkerChangeEpoch)
		} else {
			fmt.Fprintf(cctx.App.Writer, "Call 'confirm-change-worker' at or after height %d to complete.\n", mi.WorkerChangeEpoch)
		}

		return nil
	},
}

var actorConfirmChangeWorker = &cli.Command{
	Name:      "confirm-change",
	Usage:     "Confirm a worker address change",
	ArgsUsage: "[address]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name: "miner",
		},
		&cli.BoolFlag{
			Name:  "really-do-it",
			Usage: "Actually send transaction performing the action",
			Value: false,
		},
	},
	Action: func(cctx *cli.Context) error {
		if !cctx.Args().Present() {
			return fmt.Errorf("must pass address of new worker address")
		}

		ctx := context.Background()

		api, acloser, err := LotusClient(ctx)
		if err != nil {
			return err
		}
		defer acloser()

		na, err := address.NewFromString(cctx.Args().First())
		if err != nil {
			return err
		}

		newAddr, err := api.StateLookupID(ctx, na, types.EmptyTSK)
		if err != nil {
			return err
		}

		maddr, err := address.NewFromString(cctx.String("miner"))
		if err != nil {
			return err
		}

		mi, err := api.StateMinerInfo(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		if mi.NewWorker.Empty() {
			return fmt.Errorf("no worker key change proposed")
		} else if mi.NewWorker != newAddr {
			return fmt.Errorf("worker key %s does not match current worker key proposal %s", newAddr, mi.NewWorker)
		}

		if head, err := api.ChainHead(ctx); err != nil {
			return fmt.Errorf("failed to get the chain head: %w", err)
		} else if head.Height() < mi.WorkerChangeEpoch {
			return fmt.Errorf("worker key change cannot be confirmed until %d, current height is %d", mi.WorkerChangeEpoch, head.Height())
		}

		if !cctx.Bool("really-do-it") {
			fmt.Fprintln(cctx.App.Writer, "Pass --really-do-it to actually execute this action")
			return nil
		}

		smsg, err := api.MpoolPushMessage(ctx, &types.Message{
			From:   mi.Owner,
			To:     maddr,
			Method: builtin.MethodsMiner.ConfirmChangeWorkerAddress,
			Value:  big.Zero(),
		}, nil)
		if err != nil {
			return fmt.Errorf("mpool push: %w", err)
		}

		fmt.Fprintln(cctx.App.Writer, "Confirm Message CID:", smsg.Cid())

		// wait for it to get mined into a block
		wait, err := api.StateWaitMsg(ctx, smsg.Cid(), build.MessageConfidence, lotusapi.LookbackNoLimit, true)
		if err != nil {
			return err
		}

		// check it executed successfully
		if wait.Receipt.ExitCode != 0 {
			fmt.Fprintln(cctx.App.Writer, "Worker change failed!")
			return err
		}

		mi, err = api.StateMinerInfo(ctx, maddr, wait.TipSet)
		if err != nil {
			return err
		}
		if mi.Worker != newAddr {
			return fmt.Errorf("confirmed worker address change not reflected on chain: expected '%s', found '%s'", newAddr, mi.Worker)
		}

		return nil
	},
}

var controlCmd = &cli.Command{
	Name: "control",
	Subcommands: []*cli.Command{
		actorControlSet,
	},
}

var actorControlSet = &cli.Command{
	Name:      "set",
	Usage:     "Set control address(-es)",
	ArgsUsage: "[...address]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name: "miner",
		},
		&cli.BoolFlag{
			Name:  "really-do-it",
			Usage: "Actually send transaction performing the action",
			Value: false,
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := context.Background()

		api, acloser, err := LotusClient(ctx)
		if err != nil {
			return err
		}
		defer acloser()

		maddr, err := address.NewFromString(cctx.String("miner"))
		if err != nil {
			return err
		}

		mi, err := api.StateMinerInfo(ctx, maddr, types.EmptyTSK)
		if err != nil {
			return err
		}

		del := map[address.Address]struct{}{}
		existing := map[address.Address]struct{}{}
		for _, controlAddress := range mi.ControlAddresses {
			ka, err := api.StateAccountKey(ctx, controlAddress, types.EmptyTSK)
			if err != nil {
				return err
			}

			del[ka] = struct{}{}
			existing[ka] = struct{}{}
		}

		var toSet []address.Address

		for i, as := range cctx.Args().Slice() {
			a, err := address.NewFromString(as)
			if err != nil {
				return fmt.Errorf("parsing address %d: %w", i, err)
			}

			ka, err := api.StateAccountKey(ctx, a, types.EmptyTSK)
			if err != nil {
				return err
			}

			// make sure the address exists on chain
			_, err = api.StateLookupID(ctx, ka, types.EmptyTSK)
			if err != nil {
				return fmt.Errorf("looking up %s: %w", ka, err)
			}

			delete(del, ka)
			toSet = append(toSet, ka)
		}

		for a := range del {
			fmt.Println("Remove", a)
		}
		for _, a := range toSet {
			if _, exists := existing[a]; !exists {
				fmt.Println("Add", a)
			}
		}

		if !cctx.Bool("really-do-it") {
			fmt.Println("Pass --really-do-it to actually execute this action")
			return nil
		}

		cwp := &miner2.ChangeWorkerAddressParams{
			NewWorker:       mi.Worker,
			NewControlAddrs: toSet,
		}

		sp, err := actors.SerializeParams(cwp)
		if err != nil {
			return fmt.Errorf("serializing params: %w", err)
		}

		smsg, err := api.MpoolPushMessage(ctx, &types.Message{
			From:   mi.Owner,
			To:     maddr,
			Method: builtin.MethodsMiner.ChangeWorkerAddress,

			Value:  big.Zero(),
			Params: sp,
		}, nil)
		if err != nil {
			return fmt.Errorf("mpool push: %w", err)
		}

		fmt.Println("Message CID:", smsg.Cid())

		return nil
	},
}
