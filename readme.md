# JCORE CLI
This is a helper app for running jcore and other WordPress projects.

Because of the nature of this utility, it runs mostly synchronously. As such it might seem like "bad code", but it helps keep the codebase clean. It's OK to use promises as well, but optimally try to keep functions synchronous if all other things being equal. 
