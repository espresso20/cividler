# CivIdler
### A Golang based Incremental Game

This incremental game is a command-line application built with Go. The game simulates the development of a civilization through the accumulation and expenditure of resources.

## Game Logic

* **Start**: On game initiation, a camp is automatically created.

* **Villager Production**: The camp begins to generate villagers at a rate of 1 villager per second. The production formula is: 

  `Villager Production per Second = Number of Camps`
  
  So, if there are 5 camps, the game produces 5 villagers per second.

* **Buying Camps**: When 50 villagers have been produced, the player can opt to buy a new camp. This is executed with the command `bc X`, where X represents the number of camps the player wishes to buy. Each camp costs 50 villagers. The cost formula is: 

  `Cost of a New Camp = 50 Villagers`

  To buy n camps, the total cost would be `50 * n`.

* **Incremental Production**: Each camp added contributes an additional 1 villager per second to the production rate. For instance, with 5 camps, the civilization produces 5 villagers per second.

This application demonstrates the principles of incremental gameplay, resource management, and state persistence. Enjoy developing and expanding your civilization!
