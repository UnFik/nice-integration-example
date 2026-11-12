import type { ConfigError } from "../config";

export function ConfigErrorPage({ errors }: { errors: ConfigError[] }) {
  return (
    <main className="page narrow">
      <h1>Konfigurasi belum benar</h1>
      <p>
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
    </main>
  );
}
