package sqlite

var tables = map[string]string{
	"subnet": `CREATE TABLE IF NOT EXISTS subnet(
id INTEGER PRIMARY KEY,
gateway TEXT NOT NULL,
netmask TEXT NOT NULL,
my_address TEXT NOT NULL,
start TEXT NOT NULL UNIQUE,
end TEXT NOT NULL UNIQUE,
dns_server TEXT NOT NULL UNIQUE, 
UNIQUE(gateway, netmask)
)`,
	"lease": `CREATE TABLE IF NOT EXISTS lease(
id INTEGER PRIMARY KEY AUTOINCREMENT,
mac_address TEXT NOT NULL UNIQUE,
ip_address TEXT NOT NULL UNIQUE,
subnet_id INTEGER NOT NULL,
FOREIGN KEY(subnet_id) REFERENCES subnet(id) ON DELETE RESTRICT
)`,
}
