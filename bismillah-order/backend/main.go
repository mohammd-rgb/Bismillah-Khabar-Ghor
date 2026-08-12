package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

type MenuItem struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Price       int    `json:"price"`
	Description string `json:"description"`
}

type OrderItemReq struct {
	MenuItemID int `json:"menu_item_id"`
	Quantity   int `json:"quantity"`
}

type CreateOrderReq struct {
	TableNo int            `json:"table_no"`
	Items   []OrderItemReq `json:"items"`
}

type OrderDetail struct {
	ID          int    `json:"id"`
	MenuItemID  int    `json:"menu_item_id"`
	Name        string `json:"name"`
	Price       int    `json:"price"`
	Quantity    int    `json:"quantity"`
	Subtotal    int    `json:"subtotal"`
}

type Order struct {
	ID         int           `json:"id"`
	TableNo    int           `json:"table_no"`
	TotalPrice int           `json:"total_price"`
	Status     string        `json:"status"` // "PAID", "UNPAID"
	CreatedAt  time.Time     `json:"created_at"`
	Details    []OrderDetail `json:"details"`
}

func main() {
	var err error
	db, err = sql.Open("sqlite3", "./order.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	initDB()

	mux := http.NewServeMux()

	// API エンドポイント
	mux.HandleFunc("/api/menu", handleMenu)
	mux.HandleFunc("/api/orders", handleOrders)
	mux.HandleFunc("/api/orders/pay", handlePay)

	// EC2等の本番デプロイ時（frontend/distが存在する場合）の静的ファイル配信
	distDir := "../frontend/dist"
	if _, err := os.Stat(distDir); err == nil {
		fileServer := http.FileServer(http.Dir(distDir))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Join(distDir, r.URL.Path)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
				return
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	corsMux := corsMiddleware(mux)

	log.Println("サーバーをポート8080で起動中...")
	if err := http.ListenAndServe(":8080", corsMux); err != nil {
		log.Fatal(err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func initDB() {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS menu_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			category TEXT,
			price INTEGER,
			description TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			table_no INTEGER,
			total_price INTEGER,
			status TEXT,
			created_at DATETIME
		);`,
		`CREATE TABLE IF NOT EXISTS order_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id INTEGER,
			menu_item_id INTEGER,
			quantity INTEGER,
			price INTEGER
		);`,
	}

	for _, stmt := range statements {
		_, err := db.Exec(stmt)
		if err != nil {
			log.Fatal(err)
		}
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM menu_items").Scan(&count)
	if count == 0 {
		db.Exec(`INSERT INTO menu_items (name, category, price, description) VALUES
			('チキンビリヤニ', 'メイン', 1200, 'スパイス香る人気のバスマティライスの炊き込みご飯'),
			('マトンカレー', 'メイン', 1350, 'じっくり煮込んだコク旨マトンカレー'),
			('シークケバブ', 'サイド', 650, 'ジューシーなひき肉のスパイス焼き串'),
			('サモサ (2個)', 'サイド', 400, 'ジャガイモとスパイシー具材の包み揚げ'),
			('マンゴーラッシー', 'ドリンク', 450, '濃厚な甘さのヨーグルトドリンク'),
			('チャイ', 'ドリンク', 350, 'スパイシーで温かいミルクティー')`)
	}
}

func handleMenu(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := db.Query("SELECT id, name, category, price, description FROM menu_items")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var items []MenuItem
	for rows.Next() {
		var item MenuItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Category, &item.Price, &item.Description); err == nil {
			items = append(items, item)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func handleOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		rows, err := db.Query("SELECT id, table_no, total_price, status, created_at FROM orders ORDER BY created_at DESC")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var orders []Order
		for rows.Next() {
			var o Order
			if err := rows.Scan(&o.ID, &o.TableNo, &o.TotalPrice, &o.Status, &o.CreatedAt); err != nil {
				continue
			}

			itemRows, err := db.Query(`
				SELECT oi.id, oi.menu_item_id, m.name, oi.price, oi.quantity, (oi.price * oi.quantity) as subtotal
				FROM order_items oi
				JOIN menu_items m ON oi.menu_item_id = m.id
				WHERE oi.order_id = ?`, o.ID)

			if err == nil {
				o.Details = []OrderDetail{}
				for itemRows.Next() {
					var d OrderDetail
					if err := itemRows.Scan(&d.ID, &d.MenuItemID, &d.Name, &d.Price, &d.Quantity, &d.Subtotal); err == nil {
						o.Details = append(o.Details, d)
					}
				}
				itemRows.Close()
			}
			orders = append(orders, o)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(orders)

	} else if r.Method == "POST" {
		var req CreateOrderReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if len(req.Items) == 0 {
			http.Error(w, "注文アイテムが空です", http.StatusBadRequest)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		totalPrice := 0
		type calcItem struct {
			menuID int
			price  int
			qty    int
		}
		var list []calcItem

		for _, item := range req.Items {
			var price int
			err := tx.QueryRow("SELECT price FROM menu_items WHERE id = ?", item.MenuItemID).Scan(&price)
			if err != nil {
				tx.Rollback()
				http.Error(w, fmt.Sprintf("Menu ID %d not found", item.MenuItemID), http.StatusBadRequest)
				return
			}
			totalPrice += price * item.Quantity
			list = append(list, calcItem{menuID: item.MenuItemID, price: price, qty: item.Quantity})
		}

		res, err := tx.Exec("INSERT INTO orders (table_no, total_price, status, created_at) VALUES (?, ?, 'UNPAID', ?)",
			req.TableNo, totalPrice, time.Now())
		if err != nil {
			tx.Rollback()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		orderID, _ := res.LastInsertId()

		for _, item := range list {
			_, err := tx.Exec("INSERT INTO order_items (order_id, menu_item_id, quantity, price) VALUES (?, ?, ?, ?)",
				orderID, item.menuID, item.qty, item.price)
			if err != nil {
				tx.Rollback()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		tx.Commit()
		w.WriteHeader(http.StatusCreated)
	}
}

func handlePay(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		OrderID int `json:"order_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err := db.Exec("UPDATE orders SET status = 'PAID' WHERE id = ?", req.OrderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}