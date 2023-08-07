The packages in this directory:         
 - are the ones that may be needed by multiple packages in `ong`.             

1. The `github.com/komuw/ong/internal/clientip` package is need by both `github.com/komuw/ong/middleware` & `github.com/komuw/ong/cookie`.     
   So, we cannot create `clientip` inside `ong/middleware` since `ong/cookie` cannot import `ong/middleware`(middleware already imports cookie.)
2. The `github.com/komuw/ong/internal/octx` package is need by both `github.com/komuw/ong/log`, `github.com/komuw/ong/middleware` & `github.com/komuw/ong/server`
3. The `github.com/komuw/ong/internal/finger` package is need by both `github.com/komuw/ong/middleware` & `github.com/komuw/ong/server`
4. The `github.com/komuw/ong/internal/acme` package is need by both `github.com/komuw/ong/middleware` & `github.com/komuw/ong/server`
5. The `github.com/komuw/ong/internal/key` package is need by both `github.com/komuw/ong/middleware` & `github.com/komuw/ong/cry`
6. The `github.com/komuw/ong/internal/t` package is need by both `github.com/komuw/ong/middleware`, `github.com/komuw/ong/mux`, `github.com/komuw/ong/server`, etc
