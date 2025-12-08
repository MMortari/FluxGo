package fluxgo

import (
	"context"
	"log"

	"github.com/go-co-op/gocron/v2"
	"go.uber.org/fx"
)

type Cron struct {
	scheduler gocron.Scheduler
	jobs      []gocron.Job

	ctx    context.Context
	cancel context.CancelFunc
}

func (f *FluxGo) AddCron() *FluxGo {
	f.AddDependency(func() *Cron {
		s, err := gocron.NewScheduler()
		if err != nil {
			log.Fatal("Error to create cron scheduler:", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cron := Cron{
			scheduler: s,
			jobs:      make([]gocron.Job, 0),
			ctx:       ctx,
			cancel:    cancel,
		}

		return &cron
	})
	f.AddInvoke(func(lc fx.Lifecycle, cron *Cron) error {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				cron.scheduler.Start()
				f.log("CRON", "Started")
				return nil
			},
			OnStop: func(ctx context.Context) error {
				if err := cron.scheduler.Shutdown(); err != nil {
					return err
				}
				cron.cancel()
				f.log("CRON", "Stopped")
				return nil
			},
		})
		return nil
	})

	return f
}

func (c *Cron) Register(crontab string, fun CronHandler) error {
	j, err := c.scheduler.NewJob(
		gocron.CronJob(crontab, false),
		gocron.NewTask(fun),
	)
	if err != nil {
		return err
	}

	c.jobs = append(c.jobs, j)

	return nil
}
