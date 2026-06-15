(define greet (lambda (username) (println "Hello and welcome," username)))

(define calculate_total (lambda (price count) (* price count)))

(greet "Venkat")
(println "Your project calculation value is:" (calculate_total 45 4))