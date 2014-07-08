require "./polyglot"

class Hello < Polyglot::Responder
  
  def initialize
    super
    @method = "GET"
    @path = "hello"
  end
  
  def respond(json)
    puts "data received:"
    puts json
    content = Haml::Engine.new(File.read("hello.haml")).render(Object.new, show_me: "Hello, world!")
    html content
  end
end 

responder = Hello.new
responder.run


