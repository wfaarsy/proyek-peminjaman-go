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

	_ "github.com/jackc/pgx/v4/stdlib" // Driver untuk PostgreSQL
	"github.com/jung-kurt/gofpdf"
	_ "github.com/mattn/go-sqlite3" // Driver untuk SQLite
	"github.com/xuri/excelize/v2"
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

	// Railway menyediakan DATABASE_URL secara otomatis.
	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL != "" {
		// Lingkungan Produksi (Online)
		dbDriver = "pgx"
		connStr = databaseURL
		log.Println("INFO: Menggunakan database PostgreSQL (Produksi)")
	} else {
		// Lingkungan Pengembangan (Lokal)
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

	// Sintaks SQL ini kompatibel dengan PostgreSQL (SERIAL) dan SQLite.
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
		log.Printf("ERROR: Gagal query data: %v", err)
		http.Error(w, "Gagal memuat data", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var daftarPeminjaman []Peminjaman
	for rows.Next() {
		var p Peminjaman
		// Gunakan sql.NullTime untuk menangani tanggal NULL dari SQLite dan PostgreSQL.
		var tPinjam, tKembali sql.NullTime

		err := rows.Scan(&p.ID, &p.NamaPeminjam, &p.NamaBarang, &p.Jumlah, &tPinjam, &tKembali, &p.Status)
		if err != nil {
			log.Printf("ERROR: Gagal scan data: %v", err)
			continue
		}

		// Ubah format tanggal untuk ditampilkan di UI (02 Jan 2006)
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
		r.ParseForm()
		namaPeminjam := r.FormValue("nama_peminjam")
		namaBarang := r.FormValue("nama_barang")
		jumlah, _ := strconv.Atoi(r.FormValue("jumlah"))
		tanggalPinjam := r.FormValue("tanggal_pinjam")

		// Gunakan placeholder $N yang kompatibel dengan PostgreSQL dan SQLite modern.
		_, err := db.Exec("INSERT INTO peminjaman (nama_peminjam, nama_barang, jumlah, tanggal_pinjam, status) VALUES ($1, $2, $3, $4, $5)",
			namaPeminjam, namaBarang, jumlah, tanggalPinjam, "Dipinjam")
		if err != nil {
			log.Printf("ERROR: Gagal insert data: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	renderTemplate(w, "add.html", map[string]interface{}{"Title": "Tambah Peminjaman"})
}

func editHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))

	if r.Method == http.MethodPost {
		r.ParseForm()
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

		_, err := db.Exec("UPDATE peminjaman SET nama_peminjam=$1, nama_barang=$2, jumlah=$3, tanggal_pinjam=$4, tanggal_kembali=$5, status=$6 WHERE id=$7",
			namaPeminjam, namaBarang, jumlah, tanggalPinjam, tanggalKembaliToDB, status, id)
		if err != nil {
			log.Printf("ERROR: Gagal update data: %v", err)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	row := db.QueryRow("SELECT id, nama_peminjam, nama_barang, jumlah, tanggal_pinjam, tanggal_kembali, status FROM peminjaman WHERE id=$1", id)
	var p Peminjaman
	var tPinjam, tKembali sql.NullTime
	row.Scan(&p.ID, &p.NamaPeminjam, &p.NamaBarang, &p.Jumlah, &tPinjam, &tKembali, &p.Status)

	// Format tanggal untuk value di input form (YYYY-MM-DD)
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
	id, _ := strconv.Atoi(r.FormValue("id"))
	_, err := db.Exec("DELETE FROM peminjaman WHERE id=$1", id)
	if err != nil {
		log.Printf("ERROR: Gagal hapus data: %v", err)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func pdfReportHandler(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query("SELECT id, nama_peminjam, nama_barang, jumlah, tanggal_pinjam, status FROM peminjaman ORDER BY id ASC")
	defer rows.Close()

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "Laporan Peminjaman Barang Laboratorium")
	pdf.Ln(12)

	pdf.SetFont("Arial", "B", 10)
	headers := []string{"ID", "Nama Peminjam", "Nama Barang", "Jumlah", "Tgl. Pinjam", "Status"}
	widths := []float64{10, 50, 60, 20, 30, 25}
	for i, header := range headers {
		pdf.CellFormat(widths[i], 7, header, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 10)
	for rows.Next() {
		var p Peminjaman
		var tPinjam sql.NullTime
		rows.Scan(&p.ID, &p.NamaPeminjam, &p.NamaBarang, &p.Jumlah, &tPinjam, &p.Status)

		var tPinjamStr string
		if tPinjam.Valid {
			tPinjamStr = tPinjam.Time.Format("2006-01-02")
		}

		pdf.CellFormat(widths[0], 6, strconv.Itoa(p.ID), "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[1], 6, p.NamaPeminjam, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 6, p.NamaBarang, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[3], 6, strconv.Itoa(p.Jumlah), "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[4], 6, tPinjamStr, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[5], 6, p.Status, "1", 0, "C", false, 0, "")
		pdf.Ln(-1)
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=laporan_peminjaman.pdf")
	pdf.Output(w)
}

func excelReportHandler(w http.ResponseWriter, r *http.Request) {
	f := excelize.NewFile()
	sheetName := "Laporan Peminjaman"
	f.NewSheet(sheetName)
	f.SetActiveSheet(f.GetSheetIndex(sheetName))

	headers := []string{"ID", "Nama Peminjam", "Nama Barang", "Jumlah", "Tanggal Pinjam", "Tanggal Kembali", "Status"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	rows, _ := db.Query("SELECT id, nama_peminjam, nama_barang, jumlah, tanggal_pinjam, tanggal_kembali, status FROM peminjaman ORDER BY id ASC")
	defer rows.Close()

	rowNum := 2
	for rows.Next() {
		var p Peminjaman
		var tPinjam, tKembali sql.NullTime
		rows.Scan(&p.ID, &p.NamaPeminjam, &p.NamaBarang, &p.Jumlah, &tPinjam, &tKembali, &p.Status)

		var tPinjamStr, tKembaliStr string
		if tPinjam.Valid {
			tPinjamStr = tPinjam.Time.Format("2006-01-02")
		}
		if tKembali.Valid {
			tKembaliStr = tKembali.Time.Format("2006-01-02")
		}

		f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), p.ID)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), p.NamaPeminjam)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", rowNum), p.NamaBarang)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowNum), p.Jumlah)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowNum), tPinjamStr)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", rowNum), tKembaliStr)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", rowNum), p.Status)
		rowNum++
	}
	f.DeleteSheet("Sheet1")

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=laporan_peminjaman.xlsx")
	f.Write(w)
}

// === UTILITY & MAIN ===

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

	// Port ditentukan oleh layanan hosting melalui env var PORT.
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
