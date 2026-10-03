declare namespace App {
  interface Locals {
    // Whose session the request carries, as src/middleware.ts learned it.
    reader: { display_name: string } | null;
    // False when the server did not ask, as for a request with no cookie: such
    // a page may reach a signed-in reader from a cache.
    readerChecked: boolean;
  }
}
