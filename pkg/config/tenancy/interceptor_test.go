// Copyright (c) 2022 The Jaeger Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tenancy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/jaegertracing/jaeger/storage"
)

func TestTenancyInterceptor(t *testing.T) {
	tests := []struct {
		name          string
		tenancyConfig *TenancyConfig
		ctx           context.Context
		errMsg        string
	}{
		{
			name:          "missing tenant context",
			tenancyConfig: NewTenancyConfig(&Options{Enabled: true}),
			ctx:           context.Background(),
			errMsg:        "rpc error: code = PermissionDenied desc = missing tenant header",
		},
		{
			name:          "invalid tenant context",
			tenancyConfig: NewTenancyConfig(&Options{Enabled: true, Tenants: []string{"megacorp"}}),
			ctx:           storage.WithTenant(context.Background(), "acme"),
			errMsg:        "rpc error: code = PermissionDenied desc = unknown tenant header",
		},
		{
			name:          "valid tenant context",
			tenancyConfig: NewTenancyConfig(&Options{Enabled: true, Tenants: []string{"acme"}}),
			ctx:           storage.WithTenant(context.Background(), "acme"),
			errMsg:        "",
		},
		{
			name:          "invalid tenant header",
			tenancyConfig: NewTenancyConfig(&Options{Enabled: true, Tenants: []string{"megacorp"}}),
			ctx:           metadata.NewIncomingContext(context.Background(), map[string][]string{"x-tenant": {"acme"}}),
			errMsg:        "rpc error: code = PermissionDenied desc = unknown tenant",
		},
		{
			name:          "missing tenant header",
			tenancyConfig: NewTenancyConfig(&Options{Enabled: true, Tenants: []string{"megacorp"}}),
			ctx:           metadata.NewIncomingContext(context.Background(), map[string][]string{}),
			errMsg:        "rpc error: code = PermissionDenied desc = missing tenant header",
		},
		{
			name:          "valid tenant header",
			tenancyConfig: NewTenancyConfig(&Options{Enabled: true, Tenants: []string{"acme"}}),
			ctx:           metadata.NewIncomingContext(context.Background(), map[string][]string{"x-tenant": {"acme"}}),
			errMsg:        "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			interceptor := NewGuardingStreamInterceptor(test.tenancyConfig)
			ss := tenantedServerStream{
				context: test.ctx,
			}
			ssi := grpc.StreamServerInfo{}
			handler := func(interface{}, grpc.ServerStream) error {
				// do nothing
				return nil
			}
			err := interceptor(0, &ss, &ssi, handler)
			if test.errMsg == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, test.errMsg, err.Error())
			}
		})
	}
}
