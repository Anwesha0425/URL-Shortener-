import grpc from '@grpc/grpc-js';
import protoLoader from '@grpc/proto-loader';
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

const PROTO_PATH = path.resolve(__dirname, '../../shared/protos/auth.proto');

const packageDef = protoLoader.loadSync(PROTO_PATH, {
  keepCase:     true,
  longs:        String,
  enums:        String,
  defaults:     true,
  oneofs:       true,
});

const grpcObj = grpc.loadPackageDefinition(packageDef) as any;
const AuthServiceProto = grpcObj.auth.AuthService;

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

export function startGrpcServer(port: number = 50051): grpc.Server {
  const server = new grpc.Server();

  server.addService(AuthServiceProto.service, { VerifyToken: verifyToken });

  server.bindAsync(
    `0.0.0.0:${port}`,
    grpc.ServerCredentials.createInsecure(), // TLS handled by service mesh (Istio/Linkerd) in prod
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
