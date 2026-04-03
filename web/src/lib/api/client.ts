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
import { project } from '$lib/project.svelte';
import { onUnauthenticated } from '$lib/auth.svelte';

const projectInterceptor: Interceptor = (next) => async (req) => {
  req.header.set('X-Specgraph-Project', project.current || 'default');
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
  interceptors: [projectInterceptor, authErrorInterceptor],
});

export const graphClient = createClient(GraphService, transport);
export const specClient = createClient(SpecService, transport);
export const decisionClient = createClient(DecisionService, transport);
export const lifecycleClient = createClient(LifecycleService, transport);
export const constitutionClient = createClient(ConstitutionService, transport);
export const analyticalPassClient = createClient(AnalyticalPassService, transport);
export const sliceClient = createClient(SliceService, transport);
