The packages in this directory:         
 - are the ones that may be needed by multiple packages in `ong`.             
   eg, the `github.com/komuw/ong/internal/clientip` package is need by both `github.com/komuw/ong/middleware` & `github.com/komuw/ong/cookie`.     
   So, we cannot create `clientip` inside `ong/middleware` since `ong/cookie` cannot import `ong/middleware`(middleware already imports cookie.)