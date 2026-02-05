resource "nubes_tubulus_instance" "test1" {
  display_name = "Test 1"
  description  = "Тест простой запрос"
  instruction  = "сделай быстро"
}

resource "nubes_tubulus_instance" "test2" {
  display_name = "Test 2"
  description  = "Тест долго"
  instruction  = "пусть работает долго долго"
}

resource "nubes_tubulus_instance" "test3" {
  display_name = "Test 3"
  description  = "Тест с ошибкой"
  instruction  = "сломай его сразу"
}

resource "nubes_tubulus_instance" "test4" {
  display_name = "Test 4"
  description  = "Тест с сообщением"
  instruction  = "напиши привет"
}

resource "nubes_tubulus_instance" "test5" {
  display_name = "Test 5"
  description  = "Противоречивый"
  instruction  = "сделай быстро но долго и сломай но не ломай"
}

resource "nubes_tubulus_instance" "test6" {
  display_name = "Test 6"
  description  = "Неполный"
  instruction  = "хочу тубулус"
}

resource "nubes_tubulus_instance" "test7" {
  display_name = "Test 7"
  description  = "С опечатками"
  instruction  = "зделай нормална на 10 сикунд"
}
