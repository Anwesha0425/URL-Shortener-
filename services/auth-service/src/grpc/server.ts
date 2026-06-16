import grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import path from 'path';
import { verifyAccessToken } from '../services/auth.service';
import { logger } from '../utils/logger';

/**
 * gRPC Server — Auth Service
 *
 * Exposes the VerifyToken RPC for internal service-to-service calls.
 * Uses HTTP/2 multiplexing and Protocol Buffers for ~10x less overhead
 * than REST+JSON.
 *
 * Called by: url-service (Go gRPC client) to validate JWTs before writes.
 *
 * Why gRPC for internal, REST for external?
 *   - External: REST is browser-compatible, human-readable, easy to debug
 *   - Internal: gRPC is faster, strongly typed, no network overhead from JSON
 */

// Proto file is copied into the container at build time (see Dockerfile)
// Fallback: if proto file is not found, we define the service inline
const PROTO_PATH = path.resolve(__dirname, '../protos/auth.proto');

let AuthServiceProto: any;

try {
  const packageDef = protoLoader.loadSync(PROTO_PATH, {
    keepCase:     true,
    longs:        String,
    enums:        String,
    defaults:     true,
    oneofs:       true,
  });
  const grpcObj = grpc.loadPackageDefinition(packageDef) as any;
  AuthServiceProto = grpcObj.auth?.AuthService;
} catch (e) {
  logger.warn('Proto file not found — gRPC server will not start', { path: PROTO_PATH });
}

/**
 * VerifyToken — gRPC handler
 * Called by other services to validate a JWT without making an HTTP request.
 */
function verifyToken(
  call: grpc.ServerUnaryCall<{ access_token: string }, any>,
  callback: grpc.sendUnaryData<any>,
) {
  const { access_token } = call.request;

  try {
    const payload = verifyAccessToken(access_token);
    logger.info('gRPC VerifyToken: valid', { userId: payload.userId });

    callback(null, {
      valid:   true,
      user_id: payload.userId.toString(),
      email:   payload.email,
      tier:    payload.tier,
    });
  } catch (err) {
    logger.warn('gRPC VerifyToken: invalid token');
    callback(null, {
      valid:   false,
      user_id: '',
      email:   '',
      tier:    '',
    });
  }
}

export function startGrpcServer(port: number = 50051): grpc.Server | null {
  if (!AuthServiceProto) {
    logger.warn('gRPC server skipped — proto definition unavailable');
    return null;
  }

  const server = new grpc.Server();

  server.addService(AuthServiceProto.service, { VerifyToken: verifyToken });

  server.bindAsync(
    `0.0.0.0:${port}`,
    grpc.ServerCredentials.createInsecure(),
    (err, boundPort) => {
      if (err) {
        logger.error('gRPC server failed to start', { err });
        return;
      }
      logger.info(`gRPC server running on port ${boundPort}`);
    },
  );

  return server;
}
