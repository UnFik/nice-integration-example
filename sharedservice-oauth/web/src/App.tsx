import { useEffect, useSyncExternalStore } from "react";
import { describeLoginError, type Session } from "./session";
import { Home } from "./ui/Home";
import { Notice, Problem, SignIn } from "./ui/Screens";

export function App({ session }: { session: Session }) {
  const state = useSyncExternalStore(session.subscribe, session.getState, session.getState);

  useEffect(() => {
    void session.start(window.location.href);
  }, [session]);

  const signIn = () => session.signIn();
  switch (state.phase) {
    case "starting":
      return <Notice title="Memeriksa sesi…" />;
    case "unreachable":
      return (
        <Problem
          title="Backend nadi tidak terjangkau"
          message={state.message}
          icon="offline"
          tone="warning"
          action="Coba lagi"
          onAction={() => void session.retry()}
        />
      );
    case "signed-out":
      return <SignIn signedOut={state.signedOut} onSignIn={signIn} onSelectAccount={() => session.signIn({ selectAccount: true })} />;
    case "login-failed": {
      const failure = describeLoginError(state.loginError ?? "unknown");
      return failure.selectAccount ? (
        <Problem
          title={failure.title}
          message={failure.message}
          icon="block"
          tone="warning"
          action="Masuk dengan akun lain"
          onAction={() => session.signIn({ selectAccount: true })}
        />
      ) : (
        <Problem title={failure.title} message={failure.message} icon="alert" action="Masuk lagi" onAction={signIn} />
      );
    }
    case "ended":
      return (
        <Problem
          title="Sesi berakhir"
          message="NICE mengakhiri sesi Anda, mis. karena Anda keluar di tempat lain atau aksesnya dicabut. Masuk lagi untuk melanjutkan."
          icon="clock"
          tone="warning"
          action="Masuk"
          onAction={signIn}
        />
      );
    case "signed-in":
      return <Home session={session} state={state} />;
  }
}
