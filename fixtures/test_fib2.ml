let fibonacci n =
  let rec fib n memo =
    match n with
    | 0 | 1 -> n
    | _ ->
      match memo with
      | [] -> fib (n - 1) [1] + fib (n - 2) [1]
      | h :: _ -> h
  in
  fib n [];;
