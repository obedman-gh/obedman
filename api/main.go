package main

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const dsn = "host=192.168.56.10 user=obed password=rahasia123 dbname=obedman_db port=5432 sslmode=disable TimeZone=Asia/Jakarta"

var DB *gorm.DB

// 1. Model Guestbook (Akan otomatis jadi tabel di PostgreSQL)
type Guestbook struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Gagal konek ke database:", err)
	}
	log.Println("Database PostgreSQL Connected!")

	// 2. AutoMigrate: Bikin tabel otomatis kalau belum ada
	DB.AutoMigrate(&Guestbook{})
	log.Println("Migrasi tabel Guestbook selesai!")

	app := fiber.New()

	app.Static("/", "../dev/dist")

	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:4321", // Mengizinkan Astro mengakses API
	}))

	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "message": "Backend Go Obedman siap tempur!"})
	})

	// 3. Endpoint GET: Ambil semua pesan, urutkan dari yang terbaru
	app.Get("/api/guestbook", func(c *fiber.Ctx) error {
		var messages []Guestbook
		DB.Order("created_at desc").Find(&messages)
		return c.JSON(messages)
	})

	// 4. Endpoint POST: Simpan pesan baru
	app.Post("/api/guestbook", func(c *fiber.Ctx) error {
		entry := new(Guestbook)
		if err := c.BodyParser(entry); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Input tidak valid"})
		}
		if entry.Name == "" || entry.Message == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Nama dan pesan wajib diisi"})
		}

		DB.Create(&entry)
		return c.Status(201).JSON(entry)
	})

	log.Println("Server Go berjalan di port 3030")
	log.Fatal(app.Listen(":3030"))
}
