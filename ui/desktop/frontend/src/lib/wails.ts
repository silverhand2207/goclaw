// Type-safe wrappers for Wails Go runtime bindings
// Wails injects window.go at startup with bound Go methods

declare global {
  interface Window {
    go: {
      main: {
        App: {
          GetGatewayURL(): Promise<string>
          GetGatewayToken(): Promise<string>
          GetGatewayPort(): Promise<number>
          IsGatewayReady(): Promise<boolean>
          GetVersion(): Promise<string>
        }
      }
    }
  }
}

export const wails = {
  getGatewayURL: (): Promise<string> => window.go.main.App.GetGatewayURL(),
  getGatewayToken: (): Promise<string> => window.go.main.App.GetGatewayToken(),
  getGatewayPort: (): Promise<number> => window.go.main.App.GetGatewayPort(),
  isGatewayReady: (): Promise<boolean> => window.go.main.App.IsGatewayReady(),
  getVersion: (): Promise<string> => window.go.main.App.GetVersion(),
}
