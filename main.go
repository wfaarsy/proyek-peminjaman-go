package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os" // Diperlukan untuk membaca Environment Variable
	"path/filepath"
	"strconv"
	"time"

	"github.com/jung-kurt/gofpdf"
	// Import driver postgres dan sqlite
	_ "github.com/jackc/pgx/v4/stdlib"
	_ "github.com/mattn/go-sqlite3"
	"github.com/xuri/excelize/v2"
)

// Struct Model (Tidak ada perubahan)
type Peminjaman struct {
	ID              int
	NamaPeminjam    string
	NamaBarang      string
	Jumlah          int
	TanggalPinjam   string
	TanggalKembali  string
	Status          string
}

var db *sql.DB

// initDB sekarang bisa menangani dua jenis database
func initDB() {
	var err error
	var dbType, connStr string

	// Render.com akan menyediakan environment variable DATABASE_URL
	// Jika ada, kita gunakan PostgreSQL. Jika tidak, kita gunakan SQLite lokal.
	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL != "" {
		// Lingkungan Produksi (Online)
		dbType = "pgx"
		connStr = databaseURL
		log.Println("Menggunakan database PostgreSQL (Produksi)")
	} else {
		// Lingkungan Lokal
		dbType = "sqlite3"
		connStr = "./peminjaman.db"
		log.Println("Menggunakan database SQLite (Lokal)")
	}

	db, err = sql.Open(dbType, connStr)
	if err != nil {
		log.Fatalf("Gagal membuka database: %v", err)
	}

	// Perintah SQL untuk membuat tabel. Sintaks ini kompatibel untuk SQLite dan PostgreSQL.
	// SERIAL PRIMARY KEY untuk PostgreSQL, INTEGER PRIMARY KEY AUTOINCREMENT untuk SQLite.
	// Kita gunakan sintaks yang lebih umum.
	createTableSQL := `CREATE TABLE IF NOT EXISTS peminjaman (
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
		// Jangan panik jika tabel sudah ada, tapi log error lain.
		log.Printf("Gagal membuat tabel (mungkin sudah ada): %v", err)
	} else {
		log.Println("Tabel berhasil diperiksa/dibuat.")
	}
}

// Semua handler (index, add, edit, delete, report) tidak perlu diubah.
// Kode di bawah ini sama persis dengan versi sebelumnya.
// ... (Salin semua fungsi handler dari kode Anda sebelumnya ke sini) ...
// (indexHandler, addHandler, editHandler, deleteHandler, pdfReportHandler, excelReportHandler, renderTemplate)

// SALIN SEMUA FUNGSI HANDLER ANDA DARI KODE SEBELUMNYA KE SINI
// Contoh:
func indexHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, nama_peminjam, nama_barang, jumlah, tanggal_pinjam, tanggal_kembali, status FROM peminjaman ORDER BY id DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var daftarPeminjaman []Peminjaman
	for rows.Next() {
		var p Peminjaman
		var tanggalKembali sql.NullString
		var tanggalPinjam sql.NullTime // Gunakan NullTime untuk tanggal

		err := rows.Scan(&p.ID, &p.NamaPeminjam, &p.NamaBarang, &p.Jumlah, &tanggalPinjam, &tanggalKembali, &p.Status)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if tanggalPinjam.Valid {
			p.TanggalPinjam = tanggalPinjam.Time.Format("02 Jan 2006")
		}
		
		if tanggalKembali.Valid {
			tKembali, _ := time.Parse("2006-01-02T15:04:05Z", tanggalKembali.String)
			p.TanggalKembali = tKembali.Format("02 Jan 2006")
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
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		namaPeminjam := r.FormValue("nama_peminjam")
		namaBarang := r.FormValue("nama_barang")
		jumlah, _ := strconv.Atoi(r.FormValue("jumlah"))
		tanggalPinjam := r.FormValue("tanggal_pinjam")
		_, err = db.Exec("INSERT INTO peminjaman (nama_peminjam, nama_barang, jumlah, tanggal_pinjam, status) VALUES ($1, $2, $3, $4, $5)",
			namaPeminjam, namaBarang, jumlah, tanggalPinjam, "Dipinjam")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	renderTemplate(w, "add.html", map[string]interface{}{"Title": "Tambah Peminjaman"})
}

func editHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "ID tidak ditemukan", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}
	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	row := db.QueryRow("SELECT id, nama_peminjam, nama_barang, jumlah, tanggal_pinjam, tanggal_kembali, status FROM peminjaman WHERE id=$1", id)
	var p Peminjaman
	var tanggalKembali sql.NullString
	var tanggalPinjam sql.NullString
	err = row.Scan(&p.ID, &p.NamaPeminjam, &p.NamaBarang, &p.Jumlah, &tanggalPinjam, &tanggalKembali, &p.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tanggalPinjam.Valid {
		p.TanggalPinjam = tanggalPinjam.String
	}
	if tanggalKembali.Valid {
		p.TanggalKembali = tanggalKembali.String
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

	idStr := r.FormValue("id")
	if idStr == "" {
		http.Error(w, "ID tidak ditemukan", http.StatusBadRequest)
		return
	}

	_, err := db.Exec("DELETE FROM peminjaman WHERE id=$1", idStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func pdfReportHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, nama_peminjam, nama_barang, jumlah, tanggal_pinjam, status FROM peminjaman ORDER BY id ASC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "Laporan Peminjaman Barang Laboratorium")
	pdf.Ln(12)

	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(10, 7, "ID", "1", 0, "C", false, 0, "")
	pdf.CellFormat(50, 7, "Nama Peminjam", "1", 0, "C", false, 0, "")
	pdf.CellFormat(60, 7, "Nama Barang", "1", 0, "C", false, 0, "")
	pdf.CellFormat(20, 7, "Jumlah", "1", 0, "C", false, 0, "")
	pdf.CellFormat(30, 7, "Tgl. Pinjam", "1", 0, "C", false, 0, "")
	pdf.CellFormat(25, 7, "Status", "1", 0, "C", false, 0, "")
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 10)
	for rows.Next() {
		var p Peminjaman
		var tPinjam time.Time
		err := rows.Scan(&p.ID, &p.NamaPeminjam, &p.NamaBarang, &p.Jumlah, &tPinjam, &p.Status)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		formattedDate := tPinjam.Format("02-01-2006")

		pdf.CellFormat(10, 6, strconv.Itoa(p.ID), "1", 0, "C", false, 0, "")
		pdf.CellFormat(50, 6, p.NamaPeminjam, "1", 0, "L", false, 0, "")
		pdf.CellFormat(60, 6, p.NamaBarang, "1", 0, "L", false, 0, "")
		pdf.CellFormat(20, 6, strconv.Itoa(p.Jumlah), "1", 0, "C", false, 0, "")
		pdf.CellFormat(30, 6, formattedDate, "1", 0, "C", false, 0, "")
		pdf.CellFormat(25, 6, p.Status, "1", 0, "C", false, 0, "")
		pdf.Ln(-1)
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=laporan_peminjaman.pdf")
	err = pdf.Output(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func excelReportHandler(w http.ResponseWriter, r *http.Request) {
	f := excelize.NewFile()
	sheetName := "Laporan Peminjaman"
	index, _ := f.NewSheet(sheetName)
	f.SetActiveSheet(index)

	headers := []string{"ID", "Nama Peminjam", "Nama Barang", "Jumlah", "Tanggal Pinjam", "Tanggal Kembali", "Status"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	rows, err := db.Query("SELECT id, nama_peminjam, nama_barang, jumlah, tanggal_pinjam, tanggal_kembali, status FROM peminjaman ORDER BY id ASC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	rowNum := 2
	for rows.Next() {
		var p Peminjaman
		var tanggalKembali sql.NullString
		var tanggalPinjam sql.NullTime
		err := rows.Scan(&p.ID, &p.NamaPeminjam, &p.NamaBarang, &p.Jumlah, &tanggalPinjam, &tanggalKembali, &p.Status)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if tanggalPinjam.Valid {
			p.TanggalPinjam = tanggalPinjam.Time.Format("2006-01-02")
		}
		if tanggalKembali.Valid {
			p.TanggalKembali = tanggalKembali.String
		}

		f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), p.ID)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), p.NamaPeminjam)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", rowNum), p.NamaBarang)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowNum), p.Jumlah)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowNum), p.TanggalPinjam)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", rowNum), p.TanggalKembali)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", rowNum), p.Status)
		rowNum++
	}

	f.DeleteSheet("Sheet1")

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=laporan_peminjaman.xlsx")
	if err := f.Write(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderTemplate(w http.ResponseWriter, tmplName string, data interface{}) {
	tmplFiles := []string{
		filepath.Join("templates", "layout.html"),
		filepath.Join("templates", tmplName),
	}

	tmpl, err := template.ParseFiles(tmplFiles...)
	if err != nil {
		http.Error(w, "Gagal parsing template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, "Gagal render template: "+err.Error(), http.StatusInternalServerError)
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
	mux.HandleFunc("/report/pdf", pdfReportHandler)
	mux.HandleFunc("/report/excel", excelReportHandler)

	// Port ditentukan oleh Render.com melalui environment variable PORT
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Port default untuk lokal
	}

	log.Printf("Server berjalan di http://localhost:%s\n", port)
	err := http.ListenAndServe(":"+port, mux)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
