package main

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/jung-kurt/gofpdf"
	_ "github.com/mattn/go-sqlite3"
	"github.com/xuri/excelize/v2"
)

var (
	userDB      *sql.DB
	apartmentDB *sql.DB
)

// User represents a user in the database
type User struct {
	ID       int
	Username string
	Password string
}

// Apartment represents an apartment entry
type Apartment struct {
	ID       string
	Owner    string
	Resident string
	SameFlag bool
}

// Initialize the SQLite databases
func initDBs() {
	var err error

	// Open user database
	userDB, err = sql.Open("sqlite3", "./app.db")
	if err != nil {
		log.Fatal("Failed to open user database:", err)
	}

	// Create users table
	createUsersTable := `CREATE TABLE IF NOT EXISTS users (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"username" TEXT UNIQUE,
		"password" TEXT
	);`

	_, err = userDB.Exec(createUsersTable)
	if err != nil {
		log.Fatal("Failed to create users table:", err)
	}

	// Open apartment database
	apartmentDB, err = sql.Open("sqlite3", "./resident.db")
	if err != nil {
		log.Fatal("Failed to open apartment database:", err)
	}

	// Create apartments table
	createApartmentsTable := `CREATE TABLE IF NOT EXISTS apartments (
		"id" TEXT PRIMARY KEY,
		"owner" TEXT NOT NULL,
		"resident" TEXT NOT NULL,
		"same_flag" INTEGER NOT NULL
	);`

	_, err = apartmentDB.Exec(createApartmentsTable)
	if err != nil {
		log.Fatal("Failed to create apartments table:", err)
	}

	// Create collections table
	createCollectionsTable := `CREATE TABLE IF NOT EXISTS collections (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "apartment_id" TEXT NOT NULL,
    "month" TEXT NOT NULL,
    "type" TEXT NOT NULL,
    "price" REAL NOT NULL,
    "date" TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (apartment_id) REFERENCES apartments (id)
);`

	_, err = apartmentDB.Exec(createCollectionsTable)
	if err != nil {
		log.Fatal("Failed to create collections table:", err)
	}

	// Create payments table
	createPaymentsTable := `CREATE TABLE IF NOT EXISTS payments (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "month" TEXT NOT NULL,
    "type" TEXT NOT NULL,
    "price" REAL NOT NULL,
    "transaction_type" TEXT NOT NULL,
    "date" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`

	_, err = apartmentDB.Exec(createPaymentsTable)
	if err != nil {
		log.Fatal("Failed to create payments table:", err)
	}

	fmt.Println("Database init")
}

// Authentication functions
func Authenticate(username, password string) bool {
	var dbPassword string
	err := userDB.QueryRow("SELECT password FROM users WHERE username = ?", username).Scan(&dbPassword)
	if err != nil {
		log.Println("Authentication failed:", err)
		return false
	}
	return password == dbPassword
}

// Login Window
func ShowLoginWindow(myApp fyne.App) {
	loginWindow := myApp.NewWindow("Login")
	loginWindow.Resize(fyne.NewSize(400, 300))

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Username")

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Password")

	loginButton := widget.NewButton("Login", func() {
		username := usernameEntry.Text
		password := passwordEntry.Text

		if Authenticate(username, password) {
			loginWindow.Hide()
			ShowHomePage(myApp)
		} else {
			dialog.ShowError(errors.New("invalid credentials"), loginWindow)
		}
	})

	content := container.NewVBox(
		widget.NewLabel("Apartment Management System"),
		widget.NewLabel("Username:"),
		usernameEntry,
		widget.NewLabel("Password:"),
		passwordEntry,
		loginButton,
	)

	loginWindow.SetContent(content)
	loginWindow.Show()
}

// Home Page UI
func ShowHomePage(myApp fyne.App) {
	homeWindow := myApp.NewWindow("Home")
	homeWindow.Resize(fyne.NewSize(400, 300))

	userManagerButton := widget.NewButton("USER MANAGER", func() {
		homeWindow.Hide()
		ShowUserManager(myApp, homeWindow)
	})

	apartmentManagerButton := widget.NewButton("APARTMENT MANAGER", func() {
		homeWindow.Hide()
		ShowApartmentManager(myApp, homeWindow)
	})

	collectionManagerButton := widget.NewButton("COLLECTION MANAGER", func() {
		homeWindow.Hide()
		ShowCollectionManager(myApp, homeWindow)
	})

	accountsManagerButton := widget.NewButton("ACCOUNTS MANAGER", func() {
		homeWindow.Hide()
		ShowAccountsManager(myApp, homeWindow)
	})

	content := container.NewVBox(
		widget.NewLabel("Welcome to Apartment Management System"),
		container.NewCenter(userManagerButton),
		container.NewCenter(apartmentManagerButton),
		container.NewCenter(collectionManagerButton),
		container.NewCenter(accountsManagerButton),
	)

	homeWindow.SetContent(content)
	homeWindow.Show()
}

// User Manager UI
func ShowUserManager(myApp fyne.App, previousWindow fyne.Window) {
	userWindow := myApp.NewWindow("User Manager")
	userWindow.Resize(fyne.NewSize(800, 600))

	// UI elements for user management
	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Username")

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Password")

	// Create list to display users
	usersList := widget.NewList(
		func() int { return getUserCount() },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			user := getUserByIndex(id)
			obj.(*widget.Label).SetText(fmt.Sprintf("ID: %d - Username: %s", user.ID, user.Username))
		},
	)

	// Refresh function
	refreshList := func() {
		usersList.Refresh()
	}

	var currentUser User

	// Handle selecting a user from the list
	usersList.OnSelected = func(id widget.ListItemID) {
		user := getUserByIndex(id)
		currentUser = user

		usernameEntry.SetText(user.Username)
		passwordEntry.SetText(user.Password)
	}

	// Form handlers
	saveButton := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		if usernameEntry.Text == "" || passwordEntry.Text == "" {
			dialog.ShowError(errors.New("username and password are required"), userWindow)
			return
		}

		currentUser.Username = usernameEntry.Text
		currentUser.Password = passwordEntry.Text

		if err := saveUser(currentUser); err != nil {
			dialog.ShowError(err, userWindow)
			return
		}

		refreshList()
		clearUserForm(usernameEntry, passwordEntry)
	})

	addButton := widget.NewButtonWithIcon("Add New", theme.ContentAddIcon(), func() {
		currentUser = User{} // Create a new user
		clearUserForm(usernameEntry, passwordEntry)
	})

	deleteButton := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
		if currentUser.ID == 0 {
			dialog.ShowError(errors.New("select a user first"), userWindow)
			return
		}

		dialog.ShowConfirm("Confirm Delete", "Delete user "+currentUser.Username+"?",
			func(ok bool) {
				if ok {
					if err := deleteUser(currentUser.ID); err != nil {
						dialog.ShowError(err, userWindow)
						return
					}
					refreshList()
					clearUserForm(usernameEntry, passwordEntry)
				}
			}, userWindow)
	})

	// Back button to return to home
	backButton := widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), func() {
		userWindow.Hide()
		previousWindow.Show()
	})

	// Search field
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search users...")

	searchEntry.OnChanged = func(text string) {
		// This would trigger a filtered refresh of the users list
		// Currently just refreshes without filtering
		refreshList()
	}

	// Layout
	form := container.NewVBox(
		widget.NewLabel("User Details"),
		widget.NewLabel("Username:"),
		usernameEntry,
		widget.NewLabel("Password:"),
		passwordEntry,
		container.NewHBox(saveButton, addButton, deleteButton),
	)

	controls := container.NewVBox(
		container.NewHBox(searchEntry, backButton),
	)

	split := container.NewHSplit(
		container.NewBorder(controls, nil, nil, nil, usersList),
		form,
	)
	split.Offset = 0.3

	userWindow.SetContent(split)
	userWindow.Show()
}

// User database operations
func saveUser(user User) error {
	var err error
	if user.ID == 0 {
		// Insert new user
		_, err = userDB.Exec(
			"INSERT INTO users (username, password) VALUES (?, ?)",
			user.Username, user.Password,
		)
	} else {
		// Update existing user
		_, err = userDB.Exec(
			"UPDATE users SET username = ?, password = ? WHERE id = ?",
			user.Username, user.Password, user.ID,
		)
	}
	return err
}

func deleteUser(id int) error {
	_, err := userDB.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}

func getUserCount() int {
	var count int
	userDB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count
}

func getUserByIndex(index int) User {
	var user User
	row := userDB.QueryRow("SELECT id, username, password FROM users LIMIT 1 OFFSET ?", index)
	row.Scan(&user.ID, &user.Username, &user.Password)
	return user
}

func clearUserForm(usernameEntry, passwordEntry *widget.Entry) {
	usernameEntry.SetText("")
	passwordEntry.SetText("")
}

// Apartment Manager UI
func ShowApartmentManager(myApp fyne.App, previousWindow ...fyne.Window) {
	mainWindow := myApp.NewWindow("Apartment Manager")
	mainWindow.Resize(fyne.NewSize(800, 600))

	var currentApartment Apartment

	// UI elements
	idEntry := widget.NewEntry()
	idEntry.SetPlaceHolder("Apartment ID")

	ownerEntry := widget.NewEntry()
	ownerEntry.SetPlaceHolder("Owner Name")

	residentEntry := widget.NewEntry()
	residentEntry.SetPlaceHolder("Resident Name")

	// Create the checkbox with handler
	sameCheck := widget.NewCheck("Owner is Resident", func(checked bool) {
		if checked {
			residentEntry.SetText(ownerEntry.Text)
			residentEntry.Disable()
		} else {
			residentEntry.Enable()
		}
	})

	ownerEntry.OnChanged = func(s string) {
		if sameCheck.Checked {
			residentEntry.SetText(s)
		}
	}

	// List widget
	apartmentsList := widget.NewList(
		func() int { return getApartmentCount() },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			apt := getApartmentByIndex(id)
			obj.(*widget.Label).SetText(fmt.Sprintf("%s: %s - %s", apt.ID, apt.Owner, apt.Resident))
		},
	)

	refreshList := func() {
		apartmentsList.Refresh()
	}

	apartmentsList.OnSelected = func(id widget.ListItemID) {
		apt := getApartmentByIndex(id)
		currentApartment = apt

		idEntry.SetText(apt.ID)
		ownerEntry.SetText(apt.Owner)
		residentEntry.SetText(apt.Resident)
		sameCheck.SetChecked(apt.SameFlag)

		if sameCheck.Checked {
			residentEntry.Disable()
		} else {
			residentEntry.Enable()
		}
	}

	// Form handlers
	saveButton := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		if idEntry.Text == "" {
			dialog.ShowError(errors.New("apartment ID is required"), mainWindow)
			return
		}

		currentApartment.ID = idEntry.Text
		currentApartment.Owner = ownerEntry.Text

		if sameCheck.Checked {
			currentApartment.Resident = currentApartment.Owner
		} else {
			currentApartment.Resident = residentEntry.Text
			if currentApartment.Resident == "" {
				currentApartment.Resident = "Vacant"
			}
		}

		updateSameFlag(&currentApartment)

		if err := saveApartment(currentApartment); err != nil {
			dialog.ShowError(err, mainWindow)
			return
		}

		refreshList()
		clearForm(idEntry, ownerEntry, residentEntry, sameCheck)
	})

	deleteButton := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
		if currentApartment.ID == "" {
			dialog.ShowError(errors.New("select an apartment first"), mainWindow)
			return
		}

		dialog.ShowConfirm("Confirm Delete", "Delete apartment "+currentApartment.ID+"?",
			func(ok bool) {
				if ok {
					if err := deleteApartment(currentApartment.ID); err != nil {
						dialog.ShowError(err, mainWindow)
						return
					}
					refreshList()
					clearForm(idEntry, ownerEntry, residentEntry, sameCheck)
				}
			}, mainWindow)
	})

	// Back button
	backButton := widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), func() {
		mainWindow.Hide()
		if len(previousWindow) > 0 {
			previousWindow[0].Show()
		}
	})

	// Import/Export handlers
	importButton := widget.NewButtonWithIcon("Import", theme.FolderOpenIcon(), func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()

			path := reader.URI().Path()
			ext := filepath.Ext(path)

			var importErr error
			switch strings.ToLower(ext) {
			case ".csv":
				importErr = importFromCSV(path, refreshList)
			case ".xlsx":
				importErr = importFromExcel(path, refreshList)
			default:
				importErr = fmt.Errorf("unsupported file type: %s", ext)
			}

			if importErr != nil {
				dialog.ShowError(importErr, mainWindow)
			} else {
				dialog.ShowInformation("Success", "Data imported", mainWindow)
				refreshList()
			}
		}, mainWindow)
		fd.Show()
	})

	exportButton := widget.NewButtonWithIcon("Export", theme.DownloadIcon(), func() {
		fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil || writer == nil {
				return
			}
			defer writer.Close()

			path := writer.URI().Path()
			ext := filepath.Ext(path)

			var exportErr error
			switch strings.ToLower(ext) {
			case ".csv":
				exportErr = exportToCSV(path)
			case ".xlsx":
				exportErr = exportToExcel(path)
			default:
				exportErr = fmt.Errorf("unsupported file type: %s", ext)
			}

			if exportErr != nil {
				dialog.ShowError(exportErr, mainWindow)
			} else {
				dialog.ShowInformation("Success", "Data exported", mainWindow)
			}
		}, mainWindow)
		fd.Show()
	})

	// Layout
	buttons := container.NewHBox(saveButton, deleteButton, importButton, exportButton)
	if len(previousWindow) > 0 {
		buttons = container.NewHBox(saveButton, deleteButton, importButton, exportButton, backButton)
	}

	form := container.NewVBox(
		widget.NewLabel("Apartment Details"),
		widget.NewLabel("Apartment ID:"),
		idEntry,
		widget.NewLabel("Owner:"),
		ownerEntry,
		widget.NewLabel("Resident:"),
		residentEntry,
		sameCheck,
		buttons,
	)

	split := container.NewHSplit(
		container.NewBorder(nil, nil, nil, nil, apartmentsList),
		form,
	)
	split.Offset = 0.3

	mainWindow.SetContent(split)
	mainWindow.Show()
}

// Collection Manager UI
func ShowCollectionManager(myApp fyne.App, previousWindow fyne.Window) {
	collectionWindow := myApp.NewWindow("Collection Manager")
	collectionWindow.Resize(fyne.NewSize(600, 500))

	// Get all apartment IDs for dropdown
	apartmentIDs := getApartmentIDs()

	// UI elements
	apartmentSelect := widget.NewSelect(apartmentIDs, nil)
	apartmentDetailsLabel := widget.NewLabel("")

	apartmentSelect.OnChanged = func(id string) {
		apt, err := getApartmentByID(id)
		if err != nil {
			apartmentDetailsLabel.SetText("Error: " + err.Error())
			return
		}

		details := fmt.Sprintf("Owner: %s, Resident: %s", apt.Owner, apt.Resident)
		apartmentDetailsLabel.SetText(details)
	}

	// Month dropdown
	months := []string{
		"January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December",
	}
	monthSelect := widget.NewSelect(months, nil)

	// Type dropdown
	collectionTypes := []string{"Maintenance", "Other"}
	typeSelect := widget.NewSelect(collectionTypes, nil)

	// Price field (readonly)
	priceEntry := widget.NewEntry()
	priceEntry.SetText("₹4000")
	priceEntry.Disable()

	// Process button
	processButton := widget.NewButton("Process & Generate Receipt", func() {
		if apartmentSelect.Selected == "" || monthSelect.Selected == "" || typeSelect.Selected == "" {
			dialog.ShowError(errors.New("all fields are required"), collectionWindow)
			return
		}

		err := saveCollection(apartmentSelect.Selected, monthSelect.Selected,
			typeSelect.Selected, 4000.0)
		if err != nil {
			dialog.ShowError(err, collectionWindow)
			return
		}

		dialog.ShowInformation("Success",
			"Collection recorded and receipt generated",
			collectionWindow)
	})

	// Back button
	backButton := widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), func() {
		collectionWindow.Hide()
		previousWindow.Show()
	})

	// Layout
	content := container.NewVBox(
		widget.NewLabel("Collection Manager"),
		widget.NewLabel("Apartment:"),
		apartmentSelect,
		apartmentDetailsLabel,
		widget.NewLabel("Month:"),
		monthSelect,
		widget.NewLabel("Collection Type:"),
		typeSelect,
		widget.NewLabel("Price:"),
		priceEntry,
		container.NewHBox(processButton, backButton),
	)

	collectionWindow.SetContent(content)
	collectionWindow.Show()
}

// Function to get all apartment IDs
// func getApartmentIDs() []string {
// 	var ids []string
// 	rows, err := apartmentDB.Query("SELECT id FROM apartments ORDER BY id")
// 	if err != nil {
// 		log.Println("Error fetching apartment IDs:", err)
// 		return ids
// 	}
// 	defer rows.Close()

// 	for rows.Next() {
// 		var id string
// 		if err := rows.Scan(&id); err != nil {
// 			continue
// 		}
// 		ids = append(ids, id)
// 	}
// 	return ids
// }

// Function to get apartment by ID
// func getApartmentByID(id string) (Apartment, error) {
// 	var apt Apartment
// 	var sameFlag int

// 	err := apartmentDB.QueryRow(
// 		"SELECT id, owner, resident, same_flag FROM apartments WHERE id = ?",
// 		id).Scan(&apt.ID, &apt.Owner, &apt.Resident, &sameFlag)
// 	if err != nil {
// 		return apt, err
// 	}

// 	apt.SameFlag = intToBool(sameFlag)
// 	return apt, nil
// }

// Function to save collection
func saveCollection(apartmentID, month, collectionType string, price float64) error {
	// First save to database
	result, err := apartmentDB.Exec(
		"INSERT INTO collections (apartment_id, month, type, price) VALUES (?, ?, ?, ?)",
		apartmentID, month, collectionType, price)
	if err != nil {
		return err
	}

	// Get the ID of the newly inserted record
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	// Create a Collection object
	collection := Collection{
		ID:          int(id),
		ApartmentID: apartmentID,
		Month:       month,
		Type:        collectionType,
		Price:       price,
		Date:        time.Now().Format("2006-01-02"),
	}

	// Generate the receipt PDF
	return generateReceipt(collection)
}

// Accounts Manager UI
func ShowAccountsManager(myApp fyne.App, previousWindow fyne.Window) {
	accountsWindow := myApp.NewWindow("Accounts Manager")
	accountsWindow.Resize(fyne.NewSize(600, 500))

	// UI elements
	months := []string{
		"January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December",
	}
	monthSelect := widget.NewSelect(months, nil)

	// Expense type dropdown
	expenseTypes := []string{"Security Service", "Cleaning Services", "Utilities", "Repairs"}
	typeSelect := widget.NewSelect(expenseTypes, nil)

	// Price entry
	priceEntry := widget.NewEntry()
	priceEntry.SetPlaceHolder("Amount")

	// Transaction type dropdown
	transactionTypes := []string{"Debit (Money Paid)", "Credit (Money Received)"}
	transactionSelect := widget.NewSelect(transactionTypes, nil)

	// Define transactions variable and create list widget
	var transactions []Payment
	transactionList := widget.NewList(
		func() int { return len(transactions) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			t := transactions[id]
			obj.(*widget.Label).SetText(fmt.Sprintf("%s - %s: %s ₹%.2f",
				t.Month, t.Type, t.TransactionType, t.Price))
		},
	)

	// Function to refresh transaction list
	refreshTransactionList := func() {
		transactions = getRecentTransactions(10)
		transactionList.Refresh()
	}

	// Initial loading of transactions
	refreshTransactionList()

	// Process button
	processButton := widget.NewButton("Confirm Transaction", func() {
		if monthSelect.Selected == "" || typeSelect.Selected == "" ||
			priceEntry.Text == "" || transactionSelect.Selected == "" {
			dialog.ShowError(errors.New("all fields are required"), accountsWindow)
			return
		}

		price, err := strconv.ParseFloat(priceEntry.Text, 64)
		if err != nil {
			dialog.ShowError(errors.New("invalid price format"), accountsWindow)
			return
		}

		transType := "Debit"
		if transactionSelect.Selected == "Credit (Money Received)" {
			transType = "Credit"
		}

		err = savePayment(monthSelect.Selected, typeSelect.Selected, price, transType)
		if err != nil {
			dialog.ShowError(err, accountsWindow)
			return
		}

		// Refresh transaction list after saving
		refreshTransactionList()

		dialog.ShowInformation("Success", "Transaction recorded successfully", accountsWindow)
		monthSelect.ClearSelected()
		typeSelect.ClearSelected()
		priceEntry.SetText("")
		transactionSelect.ClearSelected()
	})

	// Back button
	backButton := widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), func() {
		accountsWindow.Hide()
		previousWindow.Show()
	})

	// Layout
	form := container.NewVBox(
		widget.NewLabel("Accounts Manager"),
		widget.NewLabel("Month:"),
		monthSelect,
		widget.NewLabel("Expense Type:"),
		typeSelect,
		widget.NewLabel("Amount:"),
		priceEntry,
		widget.NewLabel("Transaction Type:"),
		transactionSelect,
		container.NewHBox(processButton, backButton),
	)

	// Create a scrollable transaction list with sufficient width
	scrollableList := container.NewScroll(transactionList)
	scrollableList.SetMinSize(fyne.NewSize(1000, 300)) // Width enough to show full transaction details

	// Use the scrollable list in the history section, not the raw transactionList
	history := container.NewVBox(
		widget.NewLabel("Recent Transactions (Last 10)"),
		scrollableList,
	)

	// Use tabs for switching between form and history
	tabs := container.NewAppTabs(
		container.NewTabItem("Form", form),
		container.NewTabItem("History", history),
	)

	// Add tab change listener to refresh history when switching to that tab
	tabs.OnChanged = func(tab *container.TabItem) {
		if tab.Text == "History" {
			refreshTransactionList()
		}
	}

	content := tabs

	accountsWindow.SetContent(content)
	accountsWindow.Show()
}

// Payment struct
type Payment struct {
	ID              int
	Month           string
	Type            string
	Price           float64
	TransactionType string
	Date            string
}

// Payment database operations
func savePayment(month, expenseType string, price float64, transactionType string) error {
	_, err := apartmentDB.Exec(
		"INSERT INTO payments (month, type, price, transaction_type) VALUES (?, ?, ?, ?)",
		month, expenseType, price, transactionType)
	return err
}

func getRecentTransactions(limit int) []Payment {
	var transactions []Payment

	rows, err := apartmentDB.Query(
		`SELECT id, month, type, price, transaction_type, datetime(date) 
         FROM payments ORDER BY date DESC LIMIT ?`,
		limit)
	if err != nil {
		log.Println("Error fetching transactions:", err)
		return transactions
	}
	defer rows.Close()

	for rows.Next() {
		var t Payment
		var date string
		err := rows.Scan(&t.ID, &t.Month, &t.Type, &t.Price, &t.TransactionType, &date)
		if err != nil {
			continue
		}
		t.Date = date
		transactions = append(transactions, t)
	}
	return transactions
}

func generateReceipt(collection Collection) error {
	// Create PDF document
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "Apartment Management System")
	pdf.Ln(15)

	pdf.SetFont("Arial", "", 12)
	pdf.Cell(40, 10, fmt.Sprintf("Receipt #: %d", collection.ID))
	pdf.Ln(8)
	pdf.Cell(40, 10, fmt.Sprintf("Date: %s", collection.Date))
	pdf.Ln(8)
	pdf.Cell(40, 10, fmt.Sprintf("Apartment: %s", collection.ApartmentID))
	pdf.Ln(8)
	pdf.Cell(40, 10, fmt.Sprintf("Month: %s", collection.Month))
	pdf.Ln(8)
	pdf.Cell(40, 10, fmt.Sprintf("Type: %s", collection.Type))
	pdf.Ln(8)
	pdf.Cell(40, 10, fmt.Sprintf("Amount: ₹%.2f", collection.Price))

	// Create the output directory if it doesn't exist
	outputDir := "/home/l30/Documents/apartment_login/pdf"
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		err = os.MkdirAll(outputDir, 0o755)
		if err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Create the full file path
	filename := filepath.Join(outputDir, fmt.Sprintf("receipt_%d_%s.pdf", collection.ID, collection.ApartmentID))

	// Save the PDF
	err := pdf.OutputFileAndClose(filename)
	if err != nil {
		return fmt.Errorf("failed to save PDF file: %w", err)
	}

	fmt.Printf("PDF receipt generated: %s\n", filename)
	return nil
}

// Collection struct
type Collection struct {
	ID          int
	ApartmentID string
	Month       string
	Type        string
	Price       float64
	Date        string
}

// Function to get collection by ID
// func getCollectionByID(id int) (Collection, error) {
// 	var c Collection
// 	err := apartmentDB.QueryRow(
// 		"SELECT id, apartment_id, month, type, price, date FROM collections WHERE id = ?",
// 		id).Scan(&c.ID, &c.ApartmentID, &c.Month, &c.Type, &c.Price, &c.Date)
// 	return c, err

// 	// Layout
// 	buttons := container.NewHBox(saveButton, deleteButton, importButton, exportButton)
// 	if len(previousWindow) > 0 {
// 		buttons = container.NewHBox(saveButton, deleteButton, importButton, exportButton, backButton)
// 	}

// 	form := container.NewVBox(
// 		widget.NewLabel("Apartment Details"),
// 		widget.NewLabel("Apartment ID:"),
// 		idEntry,
// 		widget.NewLabel("Owner:"),
// 		ownerEntry,
// 		widget.NewLabel("Resident:"),
// 		residentEntry,
// 		sameCheck,
// 		buttons,
// 	)

// 	split := container.NewHSplit(
// 		container.NewBorder(nil, nil, nil, nil, apartmentsList),
// 		form,
// 	)
// 	split.Offset = 0.3

// 	mainWindow.SetContent(split)
// 	mainWindow.Show()
// }

// Apartment database operations
func saveApartment(apt Apartment) error {
	updateSameFlag(&apt)

	_, err := apartmentDB.Exec(
		`INSERT OR REPLACE INTO apartments (id, owner, resident, same_flag) 
		VALUES (?, ?, ?, ?)`,
		apt.ID, apt.Owner, apt.Resident, boolToInt(apt.SameFlag),
	)
	return err
}

func deleteApartment(id string) error {
	_, err := apartmentDB.Exec("DELETE FROM apartments WHERE id = ?", id)
	return err
}

func getApartmentCount() int {
	var count int
	apartmentDB.QueryRow("SELECT COUNT(*) FROM apartments").Scan(&count)
	return count
}

func getApartmentByIndex(index int) Apartment {
	var apt Apartment
	var sameFlag int

	row := apartmentDB.QueryRow(
		"SELECT id, owner, resident, same_flag FROM apartments LIMIT 1 OFFSET ?",
		index,
	)
	row.Scan(&apt.ID, &apt.Owner, &apt.Resident, &sameFlag)
	apt.SameFlag = intToBool(sameFlag)
	return apt
}

// Helper functions
// Add these helper functions
func getApartmentIDs() []string {
	var ids []string
	rows, err := apartmentDB.Query("SELECT id FROM apartments ORDER BY id")
	if err != nil {
		log.Println("Error fetching apartment IDs:", err)
		return ids
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func getApartmentByID(id string) (Apartment, error) {
	var apt Apartment
	var sameFlag int

	err := apartmentDB.QueryRow(
		"SELECT id, owner, resident, same_flag FROM apartments WHERE id = ?",
		id).Scan(&apt.ID, &apt.Owner, &apt.Resident, &sameFlag)
	if err != nil {
		return apt, err
	}
	apt.SameFlag = sameFlag == 1
	return apt, nil
}

func updateSameFlag(apt *Apartment) {
	apt.SameFlag = apt.Owner != "" && apt.Owner == apt.Resident
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i == 1
}

func clearForm(idEntry, ownerEntry, residentEntry *widget.Entry, sameCheck *widget.Check) {
	idEntry.SetText("")
	ownerEntry.SetText("")
	residentEntry.SetText("")
	sameCheck.SetChecked(false)
	residentEntry.Enable()
}

// Import/Export functions
func importFromCSV(path string, refresh func()) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	tx, err := apartmentDB.Begin()
	if err != nil {
		return err
	}

	for i, record := range records {
		if i == 0 { // Skip header
			continue
		}

		apt := Apartment{
			ID:       record[0],
			Owner:    record[1],
			Resident: record[2],
		}
		if apt.Resident == "" {
			apt.Resident = "Vacant"
		}
		updateSameFlag(&apt)

		_, err = tx.Exec(
			"INSERT OR REPLACE INTO apartments (id, owner, resident, same_flag) VALUES (?, ?, ?, ?)",
			apt.ID, apt.Owner, apt.Resident, boolToInt(apt.SameFlag),
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func exportToCSV(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"ID", "Owner", "Resident", "Same"}
	if err := writer.Write(header); err != nil {
		return err
	}

	rows, err := apartmentDB.Query("SELECT id, owner, resident, same_flag FROM apartments")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, owner, resident string
		var sameFlag int
		if err := rows.Scan(&id, &owner, &resident, &sameFlag); err != nil {
			return err
		}
		record := []string{id, owner, resident, fmt.Sprintf("%t", intToBool(sameFlag))}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	return nil
}

func importFromExcel(path string, refresh func()) error {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return err
	}
	defer f.Close()

	rows, err := f.GetRows("Sheet1")
	if err != nil {
		return err
	}

	tx, err := apartmentDB.Begin()
	if err != nil {
		return err
	}

	for i, row := range rows {
		if i == 0 { // Skip header
			continue
		}

		apt := Apartment{
			ID:       row[0],
			Owner:    row[1],
			Resident: row[2],
		}
		if apt.Resident == "" {
			apt.Resident = "Vacant"
		}
		updateSameFlag(&apt)

		_, err = tx.Exec(
			"INSERT OR REPLACE INTO apartments (id, owner, resident, same_flag) VALUES (?, ?, ?, ?)",
			apt.ID, apt.Owner, apt.Resident, boolToInt(apt.SameFlag),
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func exportToExcel(path string) error {
	f := excelize.NewFile()
	defer f.Close()

	// Create header
	f.SetCellValue("Sheet1", "A1", "ID")
	f.SetCellValue("Sheet1", "B1", "Owner")
	f.SetCellValue("Sheet1", "C1", "Resident")
	f.SetCellValue("Sheet1", "D1", "Same")

	rows, err := apartmentDB.Query("SELECT id, owner, resident, same_flag FROM apartments")
	if err != nil {
		return err
	}
	defer rows.Close()

	rowIdx := 2
	for rows.Next() {
		var id, owner, resident string
		var sameFlag int
		if err := rows.Scan(&id, &owner, &resident, &sameFlag); err != nil {
			return err
		}

		f.SetCellValue("Sheet1", fmt.Sprintf("A%d", rowIdx), id)
		f.SetCellValue("Sheet1", fmt.Sprintf("B%d", rowIdx), owner)
		f.SetCellValue("Sheet1", fmt.Sprintf("C%d", rowIdx), resident)
		f.SetCellValue("Sheet1", fmt.Sprintf("D%d", rowIdx), intToBool(sameFlag))
		rowIdx++
	}

	return f.SaveAs(path)
}

func main() {
	initDBs()
	myApp := app.New()
	ShowLoginWindow(myApp)
	myApp.Run()
}
