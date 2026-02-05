resource "nubes_tubulus_instance" "advanced1" {
  display_name = "Advanced 1"
  description  = "Сложный сценарий"
  instruction  = "создай на 30 секунд, пусть упадет в середине работы, в вольт запиши мой_пароль_123"
}

resource "nubes_tubulus_instance" "advanced2" {
  display_name = "Advanced 2"
  description  = "Домохозяйка 1"
  instruction  = "я не знаю что делать просто сделай что-нибудь"
}

resource "nubes_tubulus_instance" "advanced3" {
  display_name = "Advanced 3"
  description  = "Домохозяйка 2"
  instruction  = "нужно чтобы это работало хотя бы полминутки"
}

resource "nubes_tubulus_instance" "advanced4" {
  display_name = "Advanced 4"
  description  = "Технический"
  instruction  = "duration 15000ms, fail at stage 3, message: SECRET_KEY"
}

resource "nubes_tubulus_instance" "advanced5" {
  display_name = "Advanced 5"
  description  = "Смешанный стиль"
  instruction  = "поработай секунд 20, положи туда 'hello world' и вали на 1 этапе"
}
