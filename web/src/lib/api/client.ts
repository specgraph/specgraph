import { createConnectTransport } from '@connectrpc/connect-web';
import { createClient, type Interceptor } from '@connectrpc/connect';
import { ConnectError, Code } from '@connectrpc/connect';
import { GraphService } from './gen/specgraph/v1/graph_pb';
import { SpecService } from './gen/specgraph/v1/spec_pb';
import { DecisionService } from './gen/specgraph/v1/decision_pb';
import { LifecycleService } from './gen/specgraph/v1/lifecycle_pb';
import { ConstitutionService } from './gen/specgraph/v1/constitution_pb';
import { AnalyticalPassService } from './gen/specgraph/v1/analytical_pass_pb';
import { SliceService } from './gen/specgraph/v1/slice_pb';
import { IdentityService } from './gen/specgraph/v1/identity_pb';
import { project } from '$lib/project.svelte';
import { onUnauthenticated } from '$lib/auth.svelte';

const projectInterceptor: Interceptor = (next) => async (req) => {
  req.header.set('X-Specgraph-Project', project.current || 'default');
  return next(req);
};

// Name of the non-HttpOnly CSRF cookie issued by the server on the whoami GET
// (Plan 03) and the header the server's double-submit validator reads (Plan 05).
export const CSRF_COOKIE = 'specgraph_csrf';
export const CSRF_HEADER = 'X-CSRF-Token';

// readCsrfToken returns the current specgraph_csrf cookie value, or null when
// absent / running without a document (e.g. under vitest before injection).
export function readCsrfToken(): string | null {
  if (typeof document === 'undefined' || !document.cookie) return null;
  for (const part of document.cookie.split(';')) {
    const eq = part.indexOf('=');
    if (eq === -1) continue;
    if (part.slice(0, eq).trim() === CSRF_COOKIE) {
      return decodeURIComponent(part.slice(eq + 1).trim());
    }
  }
  return null;
}

// csrfInterceptor implements the D-09 double-submit defense: it echoes the
// non-HttpOnly specgraph_csrf cookie into the X-CSRF-Token header. Connect unary
// RPCs are POSTs, so cookie-authed mutations (self-mint/rotate/revoke) carry the
// token the server validates; requests without the cookie simply omit it and the
// server-side validator rejects the mutating ones.
export const csrfInterceptor: Interceptor = (next) => async (req) => {
  const token = readCsrfToken();
  if (token) req.header.set(CSRF_HEADER, token);
  return next(req);
};

const authErrorInterceptor: Interceptor = (next) => async (req) => {
  try {
    return await next(req);
  } catch (err) {
    if (err instanceof ConnectError && err.code === Code.Unauthenticated) {
      onUnauthenticated();
    }
    throw err;
  }
};

const transport = createConnectTransport({
  baseUrl: '/',
  interceptors: [projectInterceptor, csrfInterceptor, authErrorInterceptor],
});

export const graphClient = createClient(GraphService, transport);
export const specClient = createClient(SpecService, transport);
export const decisionClient = createClient(DecisionService, transport);
export const lifecycleClient = createClient(LifecycleService, transport);
export const constitutionClient = createClient(ConstitutionService, transport);
export const analyticalPassClient = createClient(AnalyticalPassService, transport);
export const sliceClient = createClient(SliceService, transport);
export const identityClient = createClient(IdentityService, transport);
