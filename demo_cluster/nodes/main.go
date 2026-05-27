package main

import (
	"cherry-game/examples/demo_cluster/nodes/web"
	"os"

	"cherry-game/examples/demo_cluster/nodes/center"
	"cherry-game/examples/demo_cluster/nodes/game"
	"cherry-game/examples/demo_cluster/nodes/gate"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:        "game cluster node",
		Description: "game cluster node examples",
		Commands: []*cli.Command{
			centerCommand(),
			gateCommand(),
			gameCommand(),
			webCommand(),
		},
	}

	_ = app.Run(os.Args)
}

func centerCommand() *cli.Command {
	return &cli.Command{
		Name:      "center",
		Usage:     "run center node",
		UsageText: "node center --path=../../config/demo-cluster.json --node=gc-center",
		Flags:     getFlag(),
		Action: func(c *cli.Context) error {
			path, node := getParameters(c)
			center.Run(path, node)
			return nil
		},
	}
}

func gateCommand() *cli.Command {
	return &cli.Command{
		Name:      "gate",
		Usage:     "run gate node",
		UsageText: "node gate --path=../../config/demo-cluster.json --node=gc-gate-1",
		Flags:     getFlag(),
		Action: func(c *cli.Context) error {
			path, node := getParameters(c)
			gate.Run(path, node)
			return nil
		},
	}
}

func gameCommand() *cli.Command {
	return &cli.Command{
		Name:      "game",
		Usage:     "run game node",
		UsageText: "node game --path=../../config/demo-cluster.json --node=10001",
		Flags:     getFlag(),
		Action: func(c *cli.Context) error {
			path, node := getParameters(c)
			game.Run(path, node)
			return nil
		},
	}
}

func webCommand() *cli.Command {
	return &cli.Command{
		Name:      "web",
		Usage:     "run web node",
		UsageText: "node web --path=./config/demo-cluster.json --node=gc-web-1",
		Flags:     getFlag(),
		Action: func(c *cli.Context) error {
			path, node := getParameters(c)
			web.Run(path, node)
			return nil
		},
	}
}

func getParameters(c *cli.Context) (path, node string) {
	path = c.String("path")
	node = c.String("node")
	return path, node
}

func getFlag() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "path",
			Usage:    "profile config path",
			Required: false,
			Value:    "../../config/demo-cluster.json",
		},
		&cli.StringFlag{
			Name:     "node",
			Usage:    "node id name",
			Required: true,
			Value:    "",
		},
	}
}
