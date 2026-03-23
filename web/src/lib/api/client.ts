import { createConnectTransport } from '@connectrpc/connect-web';
import { createClient, type Interceptor } from '@connectrpc/connect';
import { GraphService } from './gen/specgraph/v1/graph_pb';
import { SpecService } from './gen/specgraph/v1/spec_pb';
import { DecisionService } from './gen/specgraph/v1/decision_pb';
import { LifecycleService } from './gen/specgraph/v1/lifecycle_pb';

const projectInterceptor: Interceptor = (next) => async (req) => {
  req.header.set('X-Specgraph-Project', 'default');
  return next(req);
};

const transport = createConnectTransport({
  baseUrl: '/',
  interceptors: [projectInterceptor],
});

export const graphClient = createClient(GraphService, transport);
export const specClient = createClient(SpecService, transport);
export const decisionClient = createClient(DecisionService, transport);
export const lifecycleClient = createClient(LifecycleService, transport);
