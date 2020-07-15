package sqlite

var tables = map[string]string{
	"subnet": `CREATE TABLE IF NOT EXISTS subnet(
id INTEGER PRIMARY KEY,
network TEXT NOT NULL UNIQUE,
start TEXT NOT NULL UNIQUE,
end TEXT NOT NULL UNIQUE,
gateway TEXT UNIQUE,
dns_server TEXT 
)`,
	"lease": `CREATE TABLE IF NOT EXISTS lease(
id INTEGER PRIMARY KEY AUTOINCREMENT,
mac_address TEXT NOT NULL UNIQUE,
ip_address TEXT NOT NULL UNIQUE,
subnet_id INTEGER NOT NULL,
FOREIGN KEY(subnet_id) REFERENCES subnet(id) ON DELETE RESTRICT
)`,
}
