# Configuration and Replayability

This is a fun little side project for the pure joy of building something 
nostalgic.

# Problem 1: Replayability by Seed

One goal of mine in this process is to make scenes as replayable as possible.
As of today, this is done primarily by saving the seed.

However, this suffers from a fatal weakness: When the algorithms change as I 
update and add more features, the scene no longer plays out the same.

I have, so far, mitigated this to a limited extent, by avoiding changes to 
existing scene modules and making it so that each scene module starts from 
the original seed. Moving forward, though, I want the flexibility to exert 
more control over the scenes and I want to be able to make changes to the 
existing algorithms without losing replayability on older scenes.

# Problem 2: Hardcoded Configuration

Another goal is I want to expose the internal constants of scene generation 
in a configuration file. This does two things:

1. It shows me how the decisions were influenced or made by the settings 
   that were in place during a generated scene.
2. It allows me to manipulate those decisions for future scenes.

# Problem 3: Scenery Control

I would also like to encode and freeze certain aspects of a scene. For 
example, maybe I would like directly control the scenery. In fact, at some 
point, it might even be fun to make an editor: put a planet here, make the 
gas giants bands these colors, make the bands this turbulent, etc. And maybe 
let the rest of the scene remain random.

Today, I do not have anything like the ability to do this.

# Solution 0: Standard Entities

The first change needed is we need to establish the notion of a scene entity,
or entity for short. An entity is a component of the scene, visible or 
informational. Visible or informational is not a feature of the entities 
themselves, just a description of how they may be used.

An entity is formed from an instance and a schema. For now, we define the 
schema using a struct and an instance is just a object created from the 
struct.

Schemas will are only forward mutable. That is, we allow new fields to be 
added to a schema, but existing fields cannot be changed, not in name, not 
in type, and not in semantics. If a field needs to be removed or changed by a 
future version of the app in the future, we will create a new entity with a new 
schema. Fields may, however, be added to an existing entity, so long as it 
does not impact the backwards compatibility of the entity.

The name of an entity should end in `V<n>` where `<n>` is the 0-based 
version number for that schema. This way, we can replace "PlanetGasGiantV0" 
with "PlanetGasGiantV1" if we add a new feature to gas giants.

# Solution 1: Scene Algorithm Breakdown

Today, scene modules are run one after another to perform their function. 
This combines both the generation and rendering step. This has worked well 
so far, but I want to split up these responsibilities now.

## Data Breakdown: Configuration

We need to extra the configuration values for each scene module. These, with 
the seeds, are the fundamental component of the system. They can define any 
aspect of the system but typical define probabilities (the likelihood an 
event will occur in the scene) and limits (the acceptable range of lengths, 
volumes, colors, etc. that are permitted in the scene).

Generally, this information is defined as constants in the code today, but 
some decisions made inside the functions and algorithms might also be best 
pulled up into the configuration as well.

The configuration also defines the list of algorithms to use, including 
Directors, Generators, and Renderers (see below).

A configuration may either be complete or partial. That way, a user can 
define a configuration file on the local disk with only the values they care 
about. If a partial configuration is loaded, then a default configuration 
will be used to fill in all missing details. The system will always work 
with a complete configuration. Whenever writing, it will write a complete 
configuration to make results reproducible. It is permissible to load a 
partial configuration to have the missing parts filled in by the loading system.

## Data Breakdown: Globals

We need to extract all the global values that are part of the system. A 
global is a value that is derived from configuration. Example globals include 
twinkle angle, dominant sun color and phase, derived random seeds, and basicaly 
any value that needs to be selected before scene generation and rendering 
occurs.

Unlike configuration, globals are always considered complete. No field is 
permitted to be missing or set from a default or assumption.

## Data Breakdown: Entities

We need to extract the entities from the existing scene modules. Each scene 
module defines at least one entity, but some will define multiple. For 
example, because rendering of a gas giant is significantly different from 
rendering of a moon-like planet, we should define an entity for each type of 
planet. We should have a separate entity for rings, etc.

## Algorithm Breakdown: Directors

We need to extract from each scene module, and potentially other parts of 
the existing system, a component I am naming Directors. A Director is a 
function or object that takes the random seed and configuration and builds 
up the globals.

The goal is to present the generators with a cohesive unit of configuration 
that contains everything it needs in a single place to generate all the 
entities in the scene.

* Input: Seed + Instantiated Configuration
* Side-effect: None
* Output: Instantiated Globals

Seed + Complete Configuration is intended to be enough to deterministically 
instantiate an globals that are identical each run.

## Algorithm Breakdown: Generators

We need to extract from each scene module its Generator. Each Generator takes
the global values and outputs a list of entities. These entities will be
appended into the global entity list (the scene list) that comprises the 
generated scene.

While each scene module may have multiple entities, it will probably only 
have a single generator. For example, we will want the same generator to 
generate all planets, gas giant or moon-like, as they can be interleaved. If 
we did a separate generator for each, then all gas giants would be before or 
after all moon-like planets, which we don't want.

* Input: Instantiated Globals
* Side-effect: None
* Output: Scene List (list of entity instances)

Instantiated Globals with the list of Generators selected by the 
configuration should be enough to deterministically generate a
Scene List that is identical on every run.

## Algorithm Breakdown: Renderers

We need to extract from each scene module a series of Renderers. Each 
Renderer is specific to a particular entity. This is not relevant for the 
first version of this change (because we won't have enough renderers to 
matter yet), but, moving forward, no more than one Renderer may be defined 
for every entity that is in the scene list.

Renderers are not required to render anything for each entity but are the 
only thing allowed to modify the image.

* Input: Scene List (list of entity instances)
* Side-effect: Draw on the image
* Output: None

The Scene List with the list of Renderers selected by the configuration 
should be enough to deterministically draw an identical image with close to 
pixel accuracy every time.

# Solution 2: Encapsulating Configuration

As of today, the image filename contains the seed that was used to generate 
it. Moving forward, we will incorporate all the other data required to 
completely generate the data into the PNG file.

Moving forward, the system will incorporate text chunks in the PNG file, as 
defined by the PNG standard. We will embed the following text chunks:

1. `scifi-landscape/seed` - This will be a string containing the decimal 
   representation of the seed that was used for this scene.
2. `scifi-landscape/config.yaml` - This will be a complete file containing
   configuration in YAML that was used for this scene.
3. `scifi-landscape/globals.yaml` - This will be a complete file containing 
   the globals in YAML as was created by the Directors for this scene from 
   the configuration and seed.
4. `scifi-landscape/scene-list.yaml` - This will be the complete scene-list 
   in YAML as was created by the Generators for this scene from the globals. 

The PNG image itself will be the image drawn by scifi-landscape in the process.

We refer to a PNG file contianing this information as a *scene file*.

If the user passes a PNG file generated by scif-landscape, the system should 
now be able to reproduce the scene from any level:

* seed + config = Load the Directors defined in the config, recreate the 
  globals, then generate the scene list from those globals, and then render from 
  the scene list. The scene rendered on screen and any PNG output should be 
  identical, byte-for-byte, if all is implemented to meet our ideal.
* config + globals = Load the Generators from the config and use the globals to 
  build the scene list, and then render from the scene list. Same outcome.
* config + scene-list = Load the Renderers from the config and use the 
  scene-list to draw the image.

The user can specify what layer to start from. If necessary pieces are 
missing, the system will fill them in as necessary:

* *Missing Seed?* Randomly select a new seed.
* *Missing Configuration?* Create a complete configuration from the default 
  configuration.
* *Partial Configuration?* Use the default configuration to create a 
  complete configuration.
* *Missing Globals?* Use the seed and configuration to load Directors to 
  create the Globals.
* *Missing the Scene List?* Use the configuration to load Generators to create 
  the Generators and generate a scene list.

# Solution 3: Versioning Data and Algorithms

This has been partially explained already, but just to make it clear, from 
this point forward, we establish a contract that we avoid making changes to 
data structures and algorithms once the software has been released.

This means that we need a release process going forward. Once a version of 
the application is released, existing algorithms (Directors, Generators, and 
Renderers) are frozen and cannot be changed. Also existing entity schemas 
are frozen and cannot be modified (but can, in some cases, be extended to 
add new features).

New scene features can be added without worrying about versioning. But if a 
scene feature is updated, we need to incorporate strong documentation 
throughout the system to enforce this guidance: The update must be applied 
as a new revision to Director, Generator, or Renderer. We will need to 
update configuration and whatever registry of algorithms and entities that 
exist to understand this. When we can, we should try to reuse and build upon 
existing structures using Go's type system to avoid repeating ourselves, but 
we do not go back and modify existing code.

The only exception to this rule is fixing bugs that harm the performance, 
security, or cause the application to panic, segfault, or fail in other dramatic 
ways. Even in those cases, we should lean toward solutions that prevent the 
failure from impacting the operation of the app in ways that do not change 
the way it will work in normal operation cases.

We will accept the potential for drift in dependencies. To that end, the 
algorithmic code of Directors, Generators, and Renderers should prefer to 
have few dependencies, particular those that might impact the final output.

# Other Notes

The user should be able to select a seed on the command-line or in the app, as 
before. The user should be able to provide a custom configuration (partial 
or complete) on the command-line from a local file or from an existing scene 
file. The same goes for seed, globals (which will be treated as complete), and 
scene list, the user should be able to set them from local file or read 
them out of a scene file.

That means, if the user loads a scene from a scene file, they need a way to 
select which text chunks to load from the scene file to execute the first 
scene.

In the app, the configuration is locked after start, but seed, globals, and 
scene list are replaced with each new image. The user can still set the seed 
in the app.

The UI in the app should remain the same as before.

Saving a PNG file in the app will now output a complete image with the 
embedded text chunks.

A side-effect now is the system will also allow for much easier debugging 
image elements. It will just be able to construct elements in the scene-list 
and see how they render, for example.
