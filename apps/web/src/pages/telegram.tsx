import * as React from "react"
import { useNavigate, useSearchParams } from "react-router-dom"
import { api } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { useToast } from "@/hooks/use-toast"

declare global {
  interface Window {
    Telegram?: {
      WebApp?: {
        initData?: string
        ready?: () => void
        expand?: () => void
      }
    }
  }
}

function loadTelegramWebAppScript(): Promise<void> {
  return new Promise((resolve) => {
    if ((window.Telegram && window.Telegram.WebApp) || document.querySelector('script[src*="telegram-web-app.js"]')) {
      resolve()
      return
    }
    const script = document.createElement("script")
    script.src = "https://telegram.org/js/telegram-web-app.js"
    script.async = true
    script.onload = () => resolve()
    script.onerror = () => resolve()
    document.head.appendChild(script)
  })
}

export function TelegramMiniAppPage() {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const { toast } = useToast()
  const [state, setState] = React.useState<"loading" | "needLogin" | "error" | "done">("loading")
  const [error, setError] = React.useState("")

  React.useEffect(() => {
    let cancelled = false
    async function run() {
      try {
        await loadTelegramWebAppScript()
        if (cancelled) return
        const initData = window.Telegram?.WebApp?.initData || ""
        const target = params.get("mail") ? `/?mail=${encodeURIComponent(params.get("mail")!)}` : "/"
        if (!initData) {
          try {
            const me = await api.me()
            if (cancelled) return
            if (me?.user) {
              navigate(target, { replace: true })
              return
            }
          } catch {
            // not logged in
          }
          if (cancelled) return
          setState("needLogin")
          return
        }
        try {
          await api.webappAuth({ initData })
          if (cancelled) return
          setState("done")
          window.Telegram?.WebApp?.ready?.()
          window.Telegram?.WebApp?.expand?.()
          navigate(target, { replace: true })
        } catch (err) {
          if (cancelled) return
          const msg = err instanceof Error ? err.message : "登录失败"
          if (msg.includes("未绑定") || msg.includes("未配置") || msg.includes("not configured")) {
            setError(msg)
            setState("error")
            return
          }
          // 继续尝试普通登录
          try {
            const me = await api.me()
            if (cancelled) return
            if (me?.user) {
              navigate(target, { replace: true })
              return
            }
          } catch {}
          setError(msg)
          setState("error")
        }
      } catch (err) {
        if (cancelled) return
        setError(err instanceof Error ? err.message : "登录失败")
        setState("error")
      }
    }
    void run()
    return () => { cancelled = true }
  }, [navigate, params])

  if (state === "needLogin") {
    return (
      <div className="grid min-h-svh place-items-center bg-background p-6">
        <div className="w-full max-w-sm space-y-4 text-center">
          <h1 className="text-lg font-semibold">Telegram Mini App</h1>
          <p className="text-sm text-muted-foreground">请通过 Telegram 中的 Bot 命令 /open 打开此页面，或在浏览器中先登录邮箱账户。</p>
          <Button onClick={() => navigate("/login")}>去登录</Button>
        </div>
      </div>
    )
  }
  if (state === "error") {
    return (
      <div className="grid min-h-svh place-items-center bg-background p-6">
        <div className="w-full max-w-sm space-y-4 text-center">
          <h1 className="text-lg font-semibold">登录失败</h1>
          <p className="text-sm text-muted-foreground">{error}</p>
          <Button variant="outline" onClick={() => { setState("loading"); setError(""); navigate(0) }}>重试</Button>
        </div>
      </div>
    )
  }
  return <div className="grid min-h-svh place-items-center bg-background text-sm text-muted-foreground">正在登录...</div>
}
