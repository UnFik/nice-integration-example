import { useEffect, useSyncExternalStore } from "react";
import type { Session } from "./session";
import { Home } from "./ui/Home";
import { Notice, Problem, SignIn } from "./ui/Screens";

export function App({ session }: { session: Session }) {
  const state = useSyncExternalStore(session.subscribe, session.getState);

  useEffect(() => {
    if (window.location.pathname === "/callback") void session.completeLogin(window.location.href);
    else void session.resume();
  }, [session]);

  const signIn = () => void session.signIn();
  switch (state.phase) {
    case "starting":
      return <Notice title="Menghubungi NICE…" />;
    case "redirecting":
      return <Notice title="Mengalihkan ke NICE…" />;
    case "unreachable":
      return (
        <Problem
          title="NICE tidak terjangkau"
          message={state.message}
          icon="offline"
          tone="warning"
          action="Coba lagi"
          onAction={() => void session.retry()}
        />
      );
    case "signed-out":
      return <SignIn issuer={session.config.issuer} clientId={session.config.clientId} onSignIn={signIn} />;
    case "login-failed":
      return state.reason === "service_unavailable" ? (
        <Problem
          title="Service tidak tersedia"
          message={state.message}
          icon="block"
          tone="warning"
          action="Masuk dengan akun lain"
          onAction={() => void session.signIn({ selectAccount: true })}
        />
      ) : (
        <Problem title="Login gagal" message={state.message} icon="alert" action="Masuk lagi" onAction={signIn} />
      );
    case "ended":
      return <Problem title="Sesi berakhir" message={state.message} icon="clock" tone="warning" action="Masuk lagi" onAction={signIn} />;
    case "signed-in":
      return <Home session={session} state={state} />;
  }
}
