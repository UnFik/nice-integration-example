import type { ConfigError } from "../config";
import { Icon } from "./icons";

export function ConfigErrorPage({ errors }: { errors: ConfigError[] }) {
  return (
    <main className="stage">
      <div className="panel panel-wide">
        <div className="panel-icon tone-error">
          <Icon name="alert" size={26} />
        </div>
        <h1 className="panel-title">Konfigurasi belum benar</h1>
        <p className="panel-text">
          Login tidak dimulai sampai variabel berikut diperbaiki di <code>.env</code> (salin dari <code>.env.example</code>
          ), lalu <code>npm run dev</code> dijalankan ulang.
        </p>
        <ul className="errors">
          {errors.map((error, index) => (
            <li key={index}>
              <code>{error.variable}</code>: {error.message}
            </li>
          ))}
        </ul>
      </div>
    </main>
  );
}
