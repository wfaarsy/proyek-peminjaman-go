<<<<<<< HEAD
package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v4/stdlib" // Driver untuk PostgreSQL
	_ "github.com/mattn/go-sqlite3"    // Driver untuk SQLite
)

// Peminjaman merepresentasikan data di tabel.
type Peminjaman struct {
	ID             int
	NamaPeminjam   string
	NamaBarang     string
	Jumlah         int
	TanggalPinjam  string // Format untuk ditampilkan di UI atau form
	TanggalKembali string // Format untuk ditampilkan di UI atau form
	Status         string
}

var db *sql.DB

// initDB menginisialisasi koneksi ke database yang sesuai (PostgreSQL atau SQLite).
func initDB() {
	var err error
	var dbDriver, connStr string

	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL != "" {
		dbDriver = "pgx"
		connStr = databaseURL
		log.Println("INFO: Menggunakan database PostgreSQL (Produksi)")
	} else {
		dbDriver = "sqlite3"
		connStr = "./peminjaman.db" // Kita tidak lagi memerlukan ?_loc=auto
		log.Println("INFO: Menggunakan database SQLite (Lokal)")
	}

	db, err = sql.Open(dbDriver, connStr)
	if err != nil {
		log.Fatalf("FATAL: Gagal membuka database: %v", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("FATAL: Gagal terhubung ke database: %v", err)
	}

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS peminjaman (
		id SERIAL PRIMARY KEY,
		nama_peminjam TEXT,
		nama_barang TEXT,
		jumlah INTEGER,
		tanggal_pinjam DATE,
		tanggal_kembali DATE,
		status TEXT
	);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("FATAL: Gagal membuat tabel: %v", err)
	}

	log.Println("INFO: Database dan tabel berhasil diinisialisasi.")
}

// === HANDLERS ===

func indexHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, nama_peminjam, nama_barang, jumlah, tanggal_pinjam, tanggal_kembali, status FROM peminjaman ORDER BY id DESC")
	if err != nil {
		log.Printf("ERROR: Gagal query data peminjaman: %v", err)
		http.Error(w, "Gagal memuat data", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var daftarPeminjaman []Peminjaman
	for rows.Next() {
		var p Peminjaman
		// PERBAIKAN: Baca tanggal sebagai sql.NullString untuk menghindari error konversi tipe data.
		var tPinjam, tKembali sql.NullString

		err := rows.Scan(&p.ID, &p.NamaPeminjam, &p.NamaBarang, &p.Jumlah, &tPinjam, &tKembali, &p.Status)
		if err != nil {
			log.Printf("ERROR: Gagal scan baris data: %v", err)
			http.Error(w, "Gagal memproses data", http.StatusInternalServerError)
			return
		}

		// Lakukan parsing manual dari string ke format tanggal yang diinginkan.
		if tPinjam.Valid && tPinjam.String != "" {
			parsedTime, err := time.Parse("2006-01-02", tPinjam.String)
			if err == nil {
				p.TanggalPinjam = parsedTime.Format("02 Jan 2006")
			}
		}
		if tKembali.Valid && tKembali.String != "" {
			parsedTime, err := time.Parse("2006-01-02", tKembali.String)
			if err == nil {
				p.TanggalKembali = parsedTime.Format("02 Jan 2006")
			}
		} else {
			p.TanggalKembali = "-"
		}
		daftarPeminjaman = append(daftarPeminjaman, p)
	}

	data := map[string]interface{}{
		"Title":      "Daftar Peminjaman",
		"Peminjaman": daftarPeminjaman,
	}
	renderTemplate(w, "index.html", data)
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Form tidak valid", http.StatusBadRequest)
			return
		}
		namaPeminjam := r.FormValue("nama_peminjam")
		namaBarang := r.FormValue("nama_barang")
		jumlah, _ := strconv.Atoi(r.FormValue("jumlah"))
		tanggalPinjam := r.FormValue("tanggal_pinjam")

		_, err = db.Exec("INSERT INTO peminjaman (nama_peminjam, nama_barang, jumlah, tanggal_pinjam, status) VALUES ($1, $2, $3, $4, $5)",
			namaPeminjam, namaBarang, jumlah, tanggalPinjam, "Dipinjam")
		if err != nil {
			log.Printf("ERROR: Gagal insert data: %v", err)
			http.Error(w, "Gagal menyimpan data", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	renderTemplate(w, "add.html", map[string]interface{}{"Title": "Tambah Peminjaman"})
}

func editHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Form tidak valid", http.StatusBadRequest)
			return
		}
		namaPeminjam := r.FormValue("nama_peminjam")
		namaBarang := r.FormValue("nama_barang")
		jumlah, _ := strconv.Atoi(r.FormValue("jumlah"))
		tanggalPinjam := r.FormValue("tanggal_pinjam")
		tanggalKembaliStr := r.FormValue("tanggal_kembali")
		status := r.FormValue("status")
		var tanggalKembaliToDB interface{}
		if tanggalKembaliStr != "" {
			tanggalKembaliToDB = tanggalKembaliStr
			status = "Dikembalikan"
		} else {
			tanggalKembaliToDB = nil
		}
		_, err = db.Exec("UPDATE peminjaman SET nama_peminjam=$1, nama_barang=$2, jumlah=$3, tanggal_pinjam=$4, tanggal_kembali=$5, status=$6 WHERE id=$7",
			namaPeminjam, namaBarang, jumlah, tanggalPinjam, tanggalKembaliToDB, status, id)
		if err != nil {
			log.Printf("ERROR: Gagal update data ID %d: %v", id, err)
			http.Error(w, "Gagal memperbarui data", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	row := db.QueryRow("SELECT id, nama_peminjam, nama_barang, jumlah, tanggal_pinjam, tanggal_kembali, status FROM peminjaman WHERE id=$1", id)
	var p Peminjaman
	// PERBAIKAN: Baca tanggal sebagai sql.NullString.
	var tPinjam, tKembali sql.NullString

	err = row.Scan(&p.ID, &p.NamaPeminjam, &p.NamaBarang, &p.Jumlah, &tPinjam, &tKembali, &p.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		log.Printf("ERROR: Gagal scan data untuk edit ID %d: %v", id, err)
		http.Error(w, "Gagal mengambil data", http.StatusInternalServerError)
		return
	}

	// Untuk form, kita hanya perlu string YYYY-MM-DD.
	if tPinjam.Valid {
		p.TanggalPinjam = tPinjam.String
	}
	if tKembali.Valid {
		p.TanggalKembali = tKembali.String
	}

	data := map[string]interface{}{
		"Title":      "Edit Peminjaman",
		"Peminjaman": p,
	}
	renderTemplate(w, "edit.html", data)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metode tidak diizinkan", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}
	_, err = db.Exec("DELETE FROM peminjaman WHERE id=$1", id)
	if err != nil {
		log.Printf("ERROR: Gagal hapus data ID %d: %v", id, err)
		http.Error(w, "Gagal menghapus data", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ... (Handler laporan bisa disesuaikan dengan logika yang sama jika diaktifkan kembali) ...

func renderTemplate(w http.ResponseWriter, tmplName string, data interface{}) {
	tmplFiles := []string{
		filepath.Join("templates", "layout.html"),
		filepath.Join("templates", tmplName),
	}
	tmpl, err := template.ParseFiles(tmplFiles...)
	if err != nil {
		log.Printf("ERROR: Gagal parsing template %s: %v", tmplName, err)
		http.Error(w, "Gagal memuat halaman", http.StatusInternalServerError)
		return
	}
	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		log.Printf("ERROR: Gagal render template %s: %v", tmplName, err)
		http.Error(w, "Gagal menampilkan halaman", http.StatusInternalServerError)
	}
}

func main() {
	initDB()
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/add", addHandler)
	mux.HandleFunc("/edit", editHandler)
	mux.HandleFunc("/delete", deleteHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("INFO: Server berjalan di port :%s", port)
	err := http.ListenAndServe(":"+port, mux)
	if err != nil {
		log.Fatalf("FATAL: Server gagal berjalan: %v", err)
	}
}
=======
package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v4/stdlib" // Driver untuk PostgreSQL
	// "github.com/jung-kurt/gofpdf"      // PERBAIKAN: Dikomentari karena tidak digunakan
	_ "github.com/mattn/go-sqlite3" // Driver untuk SQLite
	// "github.com/xuri/excelize/v2"     // PERBAIKAN: Dikomentari karena tidak digunakan
)

// Peminjaman merepresentasikan data di tabel.
type Peminjaman struct {
	ID              int
	NamaPeminjam    string
	NamaBarang      string
	Jumlah          int
	TanggalPinjam   string // Format untuk ditampilkan di UI atau form
	TanggalKembali  string // Format untuk ditampilkan di UI atau form
	Status          string
}

var db *sql.DB

// initDB menginisialisasi koneksi ke database yang sesuai (PostgreSQL atau SQLite).
func initDB() {
	var err error
	var dbDriver, connStr string

	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL != "" {
		dbDriver = "pgx"
		connStr = databaseURL
		log.Println("INFO: Menggunakan database PostgreSQL (Produksi)")
	} else {
		dbDriver = "sqlite3"
		connStr = "./peminjaman.db?_loc=auto" // Parameter ini penting untuk SQLite lokal
		log.Println("INFO: Menggunakan database SQLite (Lokal)")
	}

	db, err = sql.Open(dbDriver, connStr)
	if err != nil {
		log.Fatalf("FATAL: Gagal membuka database: %v", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("FATAL: Gagal terhubung ke database: %v", err)
	}

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS peminjaman (
		id SERIAL PRIMARY KEY,
		nama_peminjam TEXT,
		nama_barang TEXT,
		jumlah INTEGER,
		tanggal_pinjam DATE,
		tanggal_kembali DATE,
		status TEXT
	);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("FATAL: Gagal membuat tabel: %v", err)
	}

	log.Println("INFO: Database dan tabel berhasil diinisialisasi.")
}

// === HANDLERS ===

func indexHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, nama_peminjam, nama_barang, jumlah, tanggal_pinjam, tanggal_kembali, status FROM peminjaman ORDER BY id DESC")
	if err != nil {
		log.Printf("ERROR: Gagal query data peminjaman: %v", err)
		http.Error(w, "Gagal memuat data", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var daftarPeminjaman []Peminjaman
	for rows.Next() {
		var p Peminjaman
		// PERBAIKAN FINAL: Kembali menggunakan sql.NullTime yang merupakan cara paling benar.
		var tPinjam, tKembali sql.NullTime

		err := rows.Scan(&p.ID, &p.NamaPeminjam, &p.NamaBarang, &p.Jumlah, &tPinjam, &tKembali, &p.Status)
		if err != nil {
			log.Printf("ERROR: Gagal scan baris data: %v", err)
			http.Error(w, "Gagal memproses data", http.StatusInternalServerError)
			return
		}

		// Konversi dari time.Time ke format string yang diinginkan.
		if tPinjam.Valid {
			p.TanggalPinjam = tPinjam.Time.Format("02 Jan 2006")
		}
		if tKembali.Valid {
			p.TanggalKembali = tKembali.Time.Format("02 Jan 2006")
		} else {
			p.TanggalKembali = "-"
		}
		daftarPeminjaman = append(daftarPeminjaman, p)
	}

	data := map[string]interface{}{
		"Title":      "Daftar Peminjaman",
		"Peminjaman": daftarPeminjaman,
	}
	renderTemplate(w, "index.html", data)
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Form tidak valid", http.StatusBadRequest)
			return
		}
		namaPeminjam := r.FormValue("nama_peminjam")
		namaBarang := r.FormValue("nama_barang")
		jumlah, _ := strconv.Atoi(r.FormValue("jumlah"))
		tanggalPinjam := r.FormValue("tanggal_pinjam")

		_, err = db.Exec("INSERT INTO peminjaman (nama_peminjam, nama_barang, jumlah, tanggal_pinjam, status) VALUES ($1, $2, $3, $4, $5)",
			namaPeminjam, namaBarang, jumlah, tanggalPinjam, "Dipinjam")
		if err != nil {
			log.Printf("ERROR: Gagal insert data: %v", err)
			http.Error(w, "Gagal menyimpan data", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	renderTemplate(w, "add.html", map[string]interface{}{"Title": "Tambah Peminjaman"})
}

func editHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Form tidak valid", http.StatusBadRequest)
			return
		}
		namaPeminjam := r.FormValue("nama_peminjam")
		namaBarang := r.FormValue("nama_barang")
		jumlah, _ := strconv.Atoi(r.FormValue("jumlah"))
		tanggalPinjam := r.FormValue("tanggal_pinjam")
		tanggalKembaliStr := r.FormValue("tanggal_kembali")
		status := r.FormValue("status")
		var tanggalKembaliToDB interface{}
		if tanggalKembaliStr != "" {
			tanggalKembaliToDB = tanggalKembaliStr
			status = "Dikembalikan"
		} else {
			tanggalKembaliToDB = nil
		}
		_, err = db.Exec("UPDATE peminjaman SET nama_peminjam=$1, nama_barang=$2, jumlah=$3, tanggal_pinjam=$4, tanggal_kembali=$5, status=$6 WHERE id=$7",
			namaPeminjam, namaBarang, jumlah, tanggalPinjam, tanggalKembaliToDB, status, id)
		if err != nil {
			log.Printf("ERROR: Gagal update data ID %d: %v", id, err)
			http.Error(w, "Gagal memperbarui data", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	row := db.QueryRow("SELECT id, nama_peminjam, nama_barang, jumlah, tanggal_pinjam, tanggal_kembali, status FROM peminjaman WHERE id=$1", id)
	var p Peminjaman
	// PERBAIKAN FINAL: Kembali menggunakan sql.NullTime.
	var tPinjam, tKembali sql.NullTime

	err = row.Scan(&p.ID, &p.NamaPeminjam, &p.NamaBarang, &p.Jumlah, &tPinjam, &tKembali, &p.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		log.Printf("ERROR: Gagal scan data untuk edit ID %d: %v", id, err)
		http.Error(w, "Gagal mengambil data", http.StatusInternalServerError)
		return
	}

	// Format tanggal untuk value di input form (YYYY-MM-DD).
	if tPinjam.Valid {
		p.TanggalPinjam = tPinjam.Time.Format("2006-01-02")
	}
	if tKembali.Valid {
		p.TanggalKembali = tKembali.Time.Format("2006-01-02")
	}

	data := map[string]interface{}{
		"Title":      "Edit Peminjaman",
		"Peminjaman": p,
	}
	renderTemplate(w, "edit.html", data)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metode tidak diizinkan", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}
	_, err = db.Exec("DELETE FROM peminjaman WHERE id=$1", id)
	if err != nil {
		log.Printf("ERROR: Gagal hapus data ID %d: %v", id, err)
		http.Error(w, "Gagal menghapus data", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ... (Handler laporan bisa disesuaikan dengan logika yang sama jika diaktifkan kembali) ...

func renderTemplate(w http.ResponseWriter, tmplName string, data interface{}) {
	tmplFiles := []string{
		filepath.Join("templates", "layout.html"),
		filepath.Join("templates", tmplName),
	}
	tmpl, err := template.ParseFiles(tmplFiles...)
	if err != nil {
		log.Printf("ERROR: Gagal parsing template %s: %v", tmplName, err)
		http.Error(w, "Gagal memuat halaman", http.StatusInternalServerError)
		return
	}
	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		log.Printf("ERROR: Gagal render template %s: %v", tmplName, err)
		http.Error(w, "Gagal menampilkan halaman", http.StatusInternalServerError)
	}
}

func main() {
	initDB()
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/add", addHandler)
	mux.HandleFunc("/edit", editHandler)
	mux.HandleFunc("/delete", deleteHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("INFO: Server berjalan di port :%s", port)
	err := http.ListenAndServe(":"+port, mux)
	if err != nil {
		log.Fatalf("FATAL: Server gagal berjalan: %v", err)
	}
}
>>>>>>> 62ec457a206d54644e9d898a2c7c68aab1d741d8
