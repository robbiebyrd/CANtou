package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"golang.org/x/sync/errgroup"

	cm "github.com/robbiebyrd/bb/internal/client"
	"github.com/robbiebyrd/bb/internal/client/broadcast"
	"github.com/robbiebyrd/bb/internal/config"
	canModels "github.com/robbiebyrd/bb/internal/models"
	"github.com/robbiebyrd/bb/internal/repo/csv"
	"github.com/robbiebyrd/bb/internal/repo/influxdb"
)

func main() {
	l := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	l.Info("starting application")

	cfg, cfgJson := config.Load()
	l.Debug(fmt.Sprintf("loaded config: %v", cfgJson))

	ctx := context.Background()

	l.Debug("creating channel for incoming CAN messages")
	canMsgChannel := make(chan canModels.CanMessage, cfg.MessageBufferSize)

	l.Info("creating connection manager")
	connections := cm.NewConnectionManager(&ctx, canMsgChannel, l)

	l.Info(fmt.Sprintf("creating %v can interfaces", len(cfg.CanInterfaces)))
	connections.ConnectMultiple(cfg.CanInterfaces)

	l.Info("creating database clients")
	clients := []canModels.DBClient{
		csv.NewClient(&ctx, cfg, l),
		influxdb.NewClient(&ctx, cfg, l),
	}
	l.Info(fmt.Sprintf("created %v database clients", len(clients)))

	l.Info("setting up broadcast client")
	broadcastClient := broadcast.NewBroadcastClient(&ctx, canMsgChannel)

	listeners := []broadcast.BroadcastClientListener{}
	for _, c := range clients {
		listeners = append(listeners, broadcast.BroadcastClientListener{Name: c.GetName(), Channel: c.GetChannel()})
	}

	l.Info(fmt.Sprintf("starting %v broadcast listeners", len(listeners)))
	broadcastClient.AddMany(listeners)

	l.Info("starting processes")
	var wgClients errgroup.Group

	l.Info(fmt.Sprintf("running %v clients", len(clients)))
	for _, c := range clients {
		wgClients.Go(c.Run)
		wgClients.Go(c.HandleChannel)
	}

	l.Info("starting broadcasts")
	wgClients.Go(broadcastClient.Broadcast)

	l.Info("receiving data on connections")
	wgClients.Go(connections.ReceiveAll)

	l.Info("services running, waiting for messages")
	wgClients.Wait()
}
