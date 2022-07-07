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

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/jaegertracing/jaeger/storage"
)

// tenantedServerStream is a wrapper for ServerStream providing settable context
type tenantedServerStream struct {
	grpc.ServerStream
	context context.Context
}

func (tss *tenantedServerStream) Context() context.Context {
	return tss.context
}

// NewGuardingStreamInterceptor blocks handling of streams whose tenancy header doesn't meet tenancy requirements.
// It also ensures the tenant is directly in the context, rather than context metadata.
func NewGuardingStreamInterceptor(tc *TenancyConfig) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		// Handle case where tenant is directly in the context
		tenant := storage.GetTenant(ctx)
		if tenant != "" {
			if !tc.Valid(tenant) {
				return status.Errorf(codes.PermissionDenied, "unknown tenant header")
			}
			return handler(srv, ss)
		}

		// Handle case where tenant is in the context metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return status.Errorf(codes.PermissionDenied, "missing tenant header")
		}

		var err error
		tenant, err = tenantFromMetadata(md, tc.Header)
		if err != nil {
			return err
		}
		if !tc.Valid(tenant) {
			return status.Errorf(codes.PermissionDenied, "unknown tenant")
		}

		// Apply the tenant directly the context (in addition to metadata)
		return handler(srv, &tenantedServerStream{
			ServerStream: ss,
			context:      storage.WithTenant(ctx, tenant),
		})
	}
}
