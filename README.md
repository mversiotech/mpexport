# mpexport tools

This is a set of tools to extract credentials from a
[Mooltipass](https://themooltipass.com) device and store them in a directory
hierarchy containing GPG-encrypted files in a way that is compatible with
[pass](https://passwordstore.org), the "standard unix password manager".

There are two tools, both of which communicate with the
[moolticute](https://github.com/mooltipass/moolticute) daemon over the same
websocket protocol that the mooltipass browser extension uses:

1. `csvexport` imports service and login names, but no passwords, from the
   mooltipass database and writes them to stdout in CSV format.

2. `gpgimport` reads a CSV file and then fetches the passwords for each service
   and user, one by one, from moolticute. You will need to approve the
   transmission of each password on the device.

The process is split in two parts so that you can decide which credentials to
export and where to store them. In addition to the service (host) and login
(username) columns, the CSV file can have two optional entries on each line:
The third one specifies a subdirectory in which the GPG-encrypted file should
be created. If empty, the service name will be used. The fourth, also optional
column specifies the filename for the GPG-encrypted file. If empty, the login
name will be used.
