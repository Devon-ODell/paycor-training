package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	// PostgreSQL driver
	"golang.org/x/oauth2"
)

// --- Configuration ---
// Load these from environment variables for security

var (
	paycorClientID     string
	paycorClientSecret string
	paycorRedirectURL  string
	postgresDSN        string                         // Data Source Name (e.g., "postgres://user:password@db:5432/paycordb?sslmode=disable")
	oauthStateString   = "random-string-for-security" // Use a dynamically generated, securely stored state
	oauth2Config       *oauth2.Config
	db                 *sql.DB
	// Store token temporarily in memory for this example.
	// In production, store securely (e.g., encrypted in DB or secure storage).
	accessToken *oauth2.Token
)

// --- Paycor API Specifics (PLACEHOLDERS - Get from Paycor Docs) ---
const (
	// Replace with actual Paycor Sandbox URLs
	paycorAuthURL  = "https://login-sandbox.paycor.com/oauth/authorize" // Example - Verify!
	paycorTokenURL = "https://login-sandbox.paycor.com/oauth/token"     // Example - Verify!
	// Replace with an actual Paycor Sandbox API endpoint
	paycorAPIEndpoint = "https://api-sandbox.paycor.com/v1/users/me" // Example - Verify!
)

// --- Structs for API Responses (Adapt based on actual Paycor response) ---
type PaycorUser struct {
	ID        string `json:"id"` // Example field
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	// Add other relevant fields based on the API endpoint you call
}

// --- Initialization ---
func init() {
	// Load configuration from environment variables
	paycorClientID = os.Getenv("PAYCOR_CLIENT_ID")
	paycorClientSecret = os.Getenv("PAYCOR_CLIENT_SECRET")
	paycorRedirectURL = os.Getenv("PAYCOR_REDIRECT_URL") // e.g., "http://localhost/callback" or "http://yourdomain.com/callback"
	postgresDSN = os.Getenv("POSTGRES_DSN")

	if paycorClientID == "" || paycorClientSecret == "" || paycorRedirectURL == "" || postgresDSN == "" {
		log.Fatal("Error: Required environment variables not set (PAYCOR_CLIENT_ID, PAYCOR_CLIENT_SECRET, PAYCOR_REDIRECT_URL, POSTGRES_DSN)")
	}

	// Configure OAuth2
	oauth2Config = &oauth2.Config{
		ClientID:     paycorClientID,
		ClientSecret: paycorClientSecret,
		RedirectURL:  paycorRedirectURL,
		Scopes:       []string{"openid", "profile", "offline_access", "paycor.payroll.read"}, // Adjust scopes as needed! Consult Paycor docs.
		Endpoint: oauth2.Endpoint{
			AuthURL:  paycorAuthURL,
			TokenURL: paycorTokenURL,
		},
	}

	// Initialize Database Connection
	var err error
	db, err = sql.Open("postgres", postgresDSN)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	// Ping DB to ensure connection is valid
	err = db.Ping()
	if err != nil {
		// Retry logic could be added here for Docker Compose startup timing
		log.Printf("Error pinging database, retrying in 5 seconds: %v", err)
		time.Sleep(5 * time.Second)
		err = db.Ping()
		if err != nil {
			log.Fatalf("Error pinging database after retry: %v", err)
		}
	}
	log.Println("Database connection successful!")
}

// --- HTTP Handlers ---

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if accessToken == nil || !accessToken.Valid() {
		// If not authenticated, show login link
		fmt.Fprintln(w, `<html><body>
            <h2>Paycor Integration Example</h2>
            <p>You are not authenticated.</p>
            <a href="/login">Login with Paycor Sandbox</a>
        </body></html>`)
	} else {
		// If authenticated, show option to fetch data
		fmt.Fprintln(w, `<html><body>
            <h2>Paycor Integration Example</h2>
            <p>You are authenticated!</p>
            <a href="/fetch">Fetch My User Info from Paycor</a>
        </body></html>`)
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	// Redirect user to Paycor for authorization
	url := oauth2Config.AuthCodeURL(oauthStateString) // Pass the state
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	// Handle the callback from Paycor after authorization
	state := r.FormValue("state")
	if state != oauthStateString {
		log.Printf("Invalid oauth state, expected '%s', got '%s'\n", oauthStateString, state)
		http.Error(w, "Invalid OAuth State", http.StatusBadRequest)
		return
	}

	code := r.FormValue("code")
	if code == "" {
		log.Println("OAuth code not found in callback")
		http.Error(w, "Code not found", http.StatusBadRequest)
		return
	}

	// Exchange authorization code for an access token
	token, err := oauth2Config.Exchange(context.Background(), code)
	if err != nil {
		log.Printf("oauthConfig.Exchange() failed with '%s'\n", err)
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}

	log.Printf("Access Token received (type: %s, expiry: %s)", token.TokenType, token.Expiry)
	accessToken = token // Store token (insecurely in memory for this example)

	// Redirect to the root page or a success page
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func handleFetchData(w http.ResponseWriter, r *http.Request) {
	if accessToken == nil || !accessToken.Valid() {
		log.Println("Fetch attempt without valid token, redirecting to login")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Create an authenticated HTTP client
	client := oauth2Config.Client(context.Background(), accessToken)

	// Make request to Paycor API
	resp, err := client.Get(paycorAPIEndpoint)
	if err != nil {
		log.Printf("Error making request to Paycor API: %v", err)
		http.Error(w, fmt.Sprintf("Error fetching data from Paycor: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Paycor API returned non-OK status: %s", resp.Status)
		// Read body for more details if possible
		bodyBytes, _ := io.ReadAll(resp.Body)
		http.Error(w, fmt.Sprintf("Error fetching data from Paycor: %s - %s", resp.Status, string(bodyBytes)), resp.StatusCode)
		return
	}

	// Decode the JSON response
	var paycorData PaycorUser // Use the appropriate struct based on the endpoint
	if err := json.NewDecoder(resp.Body).Decode(&paycorData); err != nil {
		log.Printf("Error decoding Paycor API response: %v", err)
		http.Error(w, fmt.Sprintf("Error decoding Paycor response: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully fetched data: %+v", paycorData)

	// --- Store data in PostgreSQL ---
	err = saveUserData(paycorData)
	if err != nil {
		log.Printf("Error saving data to database: %v", err)
		http.Error(w, fmt.Sprintf("Error saving data to database: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully saved user data for ID: %s to PostgreSQL", paycorData.ID)

	// Display fetched data (or confirmation)
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<html><body>
        <h2>Data Fetched and Stored</h2>
        <p>Successfully fetched data from Paycor and saved to PostgreSQL.</p>
        <pre>%+v</pre>
        <a href="/">Back Home</a>
    </body></html>`, paycorData)
}

// --- Database Interaction ---
func saveUserData(user PaycorUser) error {
	// Example: Insert or Update (Upsert) user data
	// Adjust table and column names based on your init.sql
	sqlStatement := `
        INSERT INTO users (paycor_id, first_name, last_name, fetched_at)
        VALUES ($1, $2, $3, NOW())
        ON CONFLICT (paycor_id) DO UPDATE SET
            first_name = EXCLUDED.first_name,
            last_name = EXCLUDED.last_name,
            fetched_at = NOW();`

	_, err := db.Exec(sqlStatement, user.ID, user.FirstName, user.LastName)
	return err // Return the error (nil if successful)
}

// --- Main Function ---
func main() {
	defer db.Close() // Ensure database connection is closed when app exits

	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/callback", handleCallback)
	http.HandleFunc("/fetch", handleFetchData) // Endpoint to trigger data fetch

	port := "8080" // Internal port the Go app listens on
	log.Printf("Go server listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
