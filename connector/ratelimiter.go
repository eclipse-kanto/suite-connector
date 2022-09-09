// Copyright (c) 2021 Contributors to the Eclipse Foundation
//
// See the NOTICE file(s) distributed with this work for additional
// information regarding copyright ownership.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0, or the Apache License, Version 2.0
// which is available at https://www.apache.org/licenses/LICENSE-2.0.
//
// SPDX-License-Identifier: EPL-2.0 OR Apache-2.0

package connector

import (
	"time"

	"golang.org/x/time/rate"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

type ratelimiter struct {
	pub message.Publisher

	limiter *rate.Limiter
}

// NewRateLimiter creates the RateLimiter as message publisher.
func NewRateLimiter(pub message.Publisher, count int64, duration time.Duration) message.Publisher {
	if pub == nil {
		panic("No Publisher")
	}

	return &ratelimiter{
		pub:     pub,
		limiter: rate.NewLimiter(rate.Every(duration/time.Duration(count)), 1),
	}
}

func (p *ratelimiter) Publish(topic string, messages ...*message.Message) error {
	var result error

	for _, msg := range messages {
		ctx := msg.Context()

		if err := p.limiter.Wait(ctx); err != nil {
			result = multierror.Append(
				result,
				errors.Errorf("publish of %s failed, message cancelled", msg.UUID),
			)
			continue
		}

		if err := p.pub.Publish(topic, msg); err != nil {
			result = multierror.Append(
				result,
				errors.Wrapf(err, "publish of %s failed", msg.UUID),
			)
		}
	}

	return result
}

func (p *ratelimiter) Close() error {
	return p.pub.Close()
}
