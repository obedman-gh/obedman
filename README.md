<div align="center">

# 🚀 Obedman — Fullstack Developer Portfolio

  <p align="center">
    A high-performance, modern fullstack developer portfolio designed for speed, aesthetics, and reliability.
    <br />
    <strong>Explore the live site »</strong>
    <br />
    <br />
    <a href="https://obedman.my.id">https://obedman.my.id</a>
  </p>

[![Astro](https://img.shields.io/badge/Astro-v5-orange.svg?style=flat-square&logo=astro&logoColor=white)](https://astro.build)
[![Go](https://img.shields.io/badge/Golang-Fiber_v2-00ADD8.svg?style=flat-square&logo=go&logoColor=white)](https://gofiber.io)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1.svg?style=flat-square&logo=postgresql&logoColor=white)](https://www.postgresql.org)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind_CSS-v4-06B6D4.svg?style=flat-square&logo=tailwindcss&logoColor=white)](https://tailwindcss.com)
[![Cloudflare](https://img.shields.io/badge/Cloudflare_Tunnel-Protected-F38020.svg?style=flat-square&logo=cloudflare&logoColor=white)](https://cloudflare.com)

</div>

---

## 📌 Overview

This repository contains the source code for **Obedman**'s personal engineering portfolio and interactive dynamic web app. Built with an **Islands Architecture** powered by **Astro** and **React**, coupled with a lightweight **Go (Golang)** backend REST API connected to a self-hosted **PostgreSQL** database.

The entire application is self-hosted on a Virtualized Ubuntu Server instance behind a secure **Cloudflare Tunnel**, providing sub-second page loads, minimal JavaScript overhead, and production-grade security.

---

## 🏗️ System Architecture

```
                                    +-------------------------------------------------+
                                    |               🌐 World Wide Web                 |
                                    +-----------------------+-------------------------+
                                                            |
                                                            v
                                            [ ☁️ Cloudflare Tunnel Edge ]
                                                            | (HTTPS Route / Port 3030)
                                                            v
+---------------------------------------------------------------------------------------------------+
| 🐧 Ubuntu Server Environment (VBox / Production Host)                                              |
|                                                                                                   |
|   +-------------------------------------------------------------------------------------------+   |
|   | 🐹 Go (Fiber) HTTP Server (:3030)                                                         |   |
|   |                                                                                           |   |
|   |   ├── 📄 Static Assets Server  ------> Serves compiled Astro HTML/CSS/JS (dist/)          |   |
|   |   └── 🔌 REST API Router       ------> /api/guestbook (GET, POST)                         |   |
|   +----------------------------------------------+--------------------------------------------+   |
|                                                  |                                                |
|                                                  v                                                |
|                                 [ 🐘 PostgreSQL Database (obedman_db) ]                          |
+---------------------------------------------------------------------------------------------------+
```

---

## ✨ Features

* **⚡ Zero-JS Default rendering:** Built with Astro for optimal core web vitals and instantaneous initial page loads.
* **🎨 Modern Dark Mode & Dynamic Spotlight:** Interactive UI with cursor-following radial gradient effects and brutalist minimalism.
* **💬 Dynamic Fullstack Guestbook:** Interactive React island connected to a Golang REST API for visitors to submit and view live feedback.
* **🔒 Self-Hosted Infrastructure:** Backend and database hosted on Ubuntu Linux with continuous Cloudflare Tunnels (no exposed public IPv4/ports).
* **📱 Fully Responsive:** Adaptive layouts supporting viewports from desktop screens to mobile devices.

---

## 🛠️ Tech Stack

### Frontend & UI
* **[Astro](https://astro.build):** Static Site Generator & Server Framework.
* **[React](https://react.dev):** Client-side interactivity (Guestbook Island).
* **[Tailwind CSS v4](https://tailwindcss.com):** Utility-first styling with modern Vite engine configuration.
* **[TypeScript](https://www.typescriptlang.org):** Type-safe frontend component logic.

### Backend & Database
* **[Go (Golang)](https://go.dev):** High-throughput, concurrent backend engine.
* **[Fiber v2](https://gofiber.io):** Express-inspired fast web framework for Go.
* **[GORM](https://gorm.io):** Developer-friendly ORM for auto-migrations and database interaction.
* **[PostgreSQL](https://www.postgresql.org):** Relational database storage.

### Infrastructure & Operations
* **OS:** Windows 11 (Development) / Ubuntu Linux (Production Guest VM).
* **Tunneling:** Cloudflare Tunnels (`cloudflared`).
* **VCS:** Git & GitHub.

---

## 📁 Repository Structure

```text
obedman/
├── api/                       # Golang REST API & Static File Server
│   ├── main.go                # Server entrypoint, Fiber routes & GORM PostgreSQL setup
│   ├── go.mod                 # Go dependencies module file
│   └── go.sum                 # Go dependencies checksums
│
└── dev/                       # Frontend Source Directory (Astro Project)
    ├── public/                # Static assets (images, avatars, icons)
    ├── src/
    │   ├── components/        # Interactive components (e.g., Guestbook.jsx)
    │   ├── layouts/           # Page layouts & HTML skeletons
    │   ├── pages/             # Astro routes & index entrypoint
    │   └── styles/            # Global CSS & Tailwind imports
    ├── astro.config.mjs       # Astro configuration
    └── package.json           # Frontend dependencies scripts
```

---

## 🚀 Local Development Setup

### Prerequisites
* **Node.js** (v18.x or higher)
* **Go** (v1.20 or higher)
* **PostgreSQL** installed and running

### 1. Clone Repository
```bash
git clone https://github.com/obedman-gh/obedman.git
cd obedman
```

### 2. Setup & Run Frontend (Astro)
```bash
cd dev
npm install
npm run dev
```
The frontend dev server will start at `http://localhost:4321`.

### 3. Setup & Run Backend (Golang API)
1. Ensure your PostgreSQL server is active and the target database (`obedman_db`) is created.
2. Update the Database DSN connection string in `api/main.go` to match your local credential settings.
3. Start the Go server:
```bash
cd ../api
go run main.go
```
The Go API & Server will start listening on `http://localhost:3030`.

---

## 🌐 Production Build & Deployment

To prepare for production deployment:

1. **Build Frontend:**
   ```bash
   cd dev
   npm run build
   ```
   This generates a static distribution folder at `dev/dist`.

2. **Serve from Go Backend:**
   Ensure `api/main.go` serves the static dist folder:
   ```go
   app.Static("/", "../dev/dist")
   ```
   Run the compiled binary:
   ```bash
   cd ../api
   go build -o server main.go
   ./server
   ```

3. **Expose with Cloudflare Tunnel:**
   ```bash
   cloudflared tunnel --url http://localhost:3030
   ```

---

## 👤 Author

**Obedman**
* **Role:** Fullstack Software Engineer
* **Website:** [https://obedman.my.id](https://obedman.my.id)
* **GitHub:** [@obedman-gh](https://github.com/obedman-gh)

---

## 📄 License

This project is open-source and available under the [MIT License](LICENSE).