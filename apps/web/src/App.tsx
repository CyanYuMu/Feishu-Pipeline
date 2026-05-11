import { createRouter, RouterProvider } from '@tanstack/react-router'
import { createRoute, createRootRoute } from '@tanstack/react-router'
import Home from './pages/Home'
import NewRequirement from './pages/NewRequirement'
import Workflows from './pages/Workflows'
import Monitoring from './pages/Monitoring'
import Delivery from './pages/Delivery'
import AuthCallback from './pages/AuthCallback'
import Session from './pages/Session'
import Debug from './pages/Debug'
import { APIDocument } from './pages/Swagger'
import Assets from './pages/Assets'
import PipelineChatPanelDemo from './pages/PipelineChatPanelDemo'
import PipelineChatPanel from './components/PipelineChatPanel'



const rootRoute = createRootRoute()

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: Home,
})

const newRequirementRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/new-requirement',
  validateSearch: (search: Record<string, unknown>): { projectId?: string } => {
    return {
      projectId: search.projectId as string | undefined,
    }
  },
  component: NewRequirement,
})

const workflowsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/workflows',
  validateSearch: (search: Record<string, unknown>): { chatDemo?: string; runId?: string } => ({
    chatDemo: search.chatDemo as string | undefined,
    runId: search.runId as string | undefined,
  }),
  component: Workflows,
})

const monitoringRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/monitoring',
  validateSearch: (search: Record<string, unknown>): { runId?: string; action?: string } => {
    return {
      runId: search.runId as string | undefined,
      action: search.action as string | undefined,
    }
  },
  component: Monitoring,
})

const deliveryRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/delivery',
  component: Delivery,
})

const approvalRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/approvals/$runId',
  component: () => (
    <div className="min-h-screen bg-slate-50">
      <PipelineChatPanel />
    </div>
  ),
})

const authCallbackRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/auth/callback',
  component: AuthCallback,
})

const sessionRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/sessions/$sessionId',
  component: Session,
})

const debugRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/debug',
  component: Debug,
})

const swaggerRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/swagger',
  component: APIDocument,
})

const assetsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/assets',
  component: Assets,
})

const pipelineChatPanelDemoRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/pipeline-chat-demo',
  validateSearch: (search: Record<string, unknown>): { chatDemo?: string } => ({
    chatDemo: search.chatDemo as string | undefined,
  }),
  component: PipelineChatPanelDemo,
})

const routeTree = rootRoute.addChildren([indexRoute, newRequirementRoute, workflowsRoute, monitoringRoute, deliveryRoute, approvalRoute, authCallbackRoute, sessionRoute, debugRoute, swaggerRoute, assetsRoute, pipelineChatPanelDemoRoute])
const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

function App() {
  return <RouterProvider router={router} />
}

export default App
