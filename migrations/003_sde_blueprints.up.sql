-- 003_sde_blueprints.up.sql
-- EVE Online SDE-compatible tables for Blueprint BOM, ME/TE calculations, and live Jita auto-costing.
-- Ships with a curated starter set of 33 beginner-to-intermediate EVE blueprints (expandable via cmd/sde-import).

CREATE TABLE IF NOT EXISTS sde_types (
    type_id INT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    group_name VARCHAR(120) NOT NULL DEFAULT '',
    category VARCHAR(80) NOT NULL DEFAULT 'Other',
    volume_m3 NUMERIC(20,4) NOT NULL DEFAULT 0.01,
    portion_size INT NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_sde_types_name ON sde_types (LOWER(name));

CREATE TABLE IF NOT EXISTS sde_blueprints (
    blueprint_type_id INT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    product_type_id INT NOT NULL,
    product_name VARCHAR(255) NOT NULL,
    product_qty_per_run INT NOT NULL DEFAULT 1,
    base_time_seconds INT NOT NULL DEFAULT 3600,
    tech_level INT NOT NULL DEFAULT 1,
    category VARCHAR(80) NOT NULL DEFAULT 'Other'
);

CREATE INDEX IF NOT EXISTS idx_sde_blueprints_name ON sde_blueprints (LOWER(name));
CREATE INDEX IF NOT EXISTS idx_sde_blueprints_product ON sde_blueprints (LOWER(product_name));
CREATE INDEX IF NOT EXISTS idx_sde_blueprints_category ON sde_blueprints (category);

CREATE TABLE IF NOT EXISTS sde_blueprint_materials (
    blueprint_type_id INT NOT NULL REFERENCES sde_blueprints(blueprint_type_id) ON DELETE CASCADE,
    material_type_id INT NOT NULL REFERENCES sde_types(type_id) ON DELETE CASCADE,
    base_quantity BIGINT NOT NULL,
    PRIMARY KEY (blueprint_type_id, material_type_id)
);

CREATE INDEX IF NOT EXISTS idx_sde_bp_materials_bp ON sde_blueprint_materials (blueprint_type_id);

-- 1. Seed Core Minerals
INSERT INTO sde_types (type_id, name, group_name, category, volume_m3, portion_size) VALUES
    (34, 'Tritanium', 'Mineral', 'Minerals', 0.01, 1),
    (35, 'Pyerite', 'Mineral', 'Minerals', 0.01, 1),
    (36, 'Mexallon', 'Mineral', 'Minerals', 0.01, 1),
    (37, 'Isogen', 'Mineral', 'Minerals', 0.01, 1),
    (38, 'Nocxium', 'Mineral', 'Minerals', 0.01, 1),
    (39, 'Zydrine', 'Mineral', 'Minerals', 0.01, 1),
    (40, 'Megacyte', 'Mineral', 'Minerals', 0.01, 1)
ON CONFLICT (type_id) DO NOTHING;

-- 2. Seed Starter Product Types
INSERT INTO sde_types (type_id, name, group_name, category, volume_m3, portion_size) VALUES
    -- Drones
    (2454, 'Hobgoblin I', 'Combat Drone', 'Drones', 5.0, 1),
    (2486, 'Warrior I', 'Combat Drone', 'Drones', 5.0, 1),
    (2464, 'Hornet I', 'Combat Drone', 'Drones', 5.0, 1),
    (2183, 'Hammerhead I', 'Combat Drone', 'Drones', 10.0, 1),
    (2173, 'Vespa I', 'Combat Drone', 'Drones', 10.0, 1),
    (2444, 'Valkyrie I', 'Combat Drone', 'Drones', 10.0, 1),
    (10246, 'Mining Drone I', 'Mining Drone', 'Drones', 5.0, 1),
    -- Ammunition
    (250, 'Antimatter Charge S', 'Hybrid Charge', 'Ammunition', 0.0025, 100),
    (258, 'Antimatter Charge M', 'Hybrid Charge', 'Ammunition', 0.0125, 100),
    (209, 'Scourge Light Missile', 'Light Missile', 'Ammunition', 0.015, 100),
    (210, 'Inferno Light Missile', 'Light Missile', 'Ammunition', 0.015, 100),
    (183, 'EMP S', 'Projectile Ammo', 'Ammunition', 0.0025, 100),
    (185, 'Phased Plasma S', 'Projectile Ammo', 'Ammunition', 0.0025, 100),
    -- Frigates & Mining
    (587, 'Rifter', 'Frigate', 'Ships', 2500.0, 1),
    (603, 'Merlin', 'Frigate', 'Ships', 2500.0, 1),
    (608, 'Atron', 'Frigate', 'Ships', 2500.0, 1),
    (593, 'Tristan', 'Frigate', 'Ships', 2500.0, 1),
    (597, 'Punisher', 'Frigate', 'Ships', 2500.0, 1),
    (32880, 'Venture', 'Mining Frigate', 'Ships', 2500.0, 1),
    (29248, 'Heron', 'Frigate', 'Ships', 2500.0, 1),
    -- Destroyers & Cruisers
    (16240, 'Catalyst', 'Destroyer', 'Ships', 5000.0, 1),
    (16242, 'Thrasher', 'Destroyer', 'Ships', 5000.0, 1),
    (16236, 'Coercer', 'Destroyer', 'Ships', 5000.0, 1),
    (16238, 'Cormorant', 'Destroyer', 'Ships', 5000.0, 1),
    (621, 'Caracal', 'Cruiser', 'Ships', 10000.0, 1),
    (626, 'Vexor', 'Cruiser', 'Ships', 10000.0, 1),
    -- Modules
    (2046, 'Damage Control I', 'Damage Control', 'Modules', 5.0, 1),
    (3839, 'Large Shield Extender I', 'Shield Extender', 'Modules', 25.0, 1),
    (518, '1MN Afterburner I', 'Propulsion Module', 'Modules', 5.0, 1),
    (526, '5MN Microwarpdrive I', 'Propulsion Module', 'Modules', 5.0, 1),
    (4391, 'Medium Shield Booster I', 'Shield Booster', 'Modules', 10.0, 1),
    (3242, 'Warp Scrambler I', 'Warp Scrambler', 'Modules', 5.0, 1),
    (527, 'Stasis Webifier I', 'Stasis Web', 'Modules', 5.0, 1)
ON CONFLICT (type_id) DO NOTHING;

-- 3. Seed Starter Blueprints (33 curated high-velocity blueprints)
INSERT INTO sde_blueprints (blueprint_type_id, name, product_type_id, product_name, product_qty_per_run, base_time_seconds, tech_level, category) VALUES
    -- Drones
    (2455, 'Hobgoblin I Blueprint', 2454, 'Hobgoblin I', 1, 1800, 1, 'Drones'),
    (2487, 'Warrior I Blueprint', 2486, 'Warrior I', 1, 1800, 1, 'Drones'),
    (2465, 'Hornet I Blueprint', 2464, 'Hornet I', 1, 1800, 1, 'Drones'),
    (2184, 'Hammerhead I Blueprint', 2183, 'Hammerhead I', 1, 3600, 1, 'Drones'),
    (2174, 'Vespa I Blueprint', 2173, 'Vespa I', 1, 3600, 1, 'Drones'),
    (2445, 'Valkyrie I Blueprint', 2444, 'Valkyrie I', 1, 3600, 1, 'Drones'),
    (10247, 'Mining Drone I Blueprint', 10246, 'Mining Drone I', 1, 1800, 1, 'Drones'),
    -- Ammunition
    (1066, 'Antimatter Charge S Blueprint', 250, 'Antimatter Charge S', 100, 600, 1, 'Ammunition'),
    (1074, 'Antimatter Charge M Blueprint', 258, 'Antimatter Charge M', 100, 900, 1, 'Ammunition'),
    (1025, 'Scourge Light Missile Blueprint', 209, 'Scourge Light Missile', 100, 900, 1, 'Ammunition'),
    (1026, 'Inferno Light Missile Blueprint', 210, 'Inferno Light Missile', 100, 900, 1, 'Ammunition'),
    (999, 'EMP S Blueprint', 183, 'EMP S', 100, 600, 1, 'Ammunition'),
    (1001, 'Phased Plasma S Blueprint', 185, 'Phased Plasma S', 100, 600, 1, 'Ammunition'),
    -- Frigates
    (786, 'Rifter Blueprint', 587, 'Rifter', 1, 4800, 1, 'Ships'),
    (802, 'Merlin Blueprint', 603, 'Merlin', 1, 4800, 1, 'Ships'),
    (807, 'Atron Blueprint', 608, 'Atron', 1, 4800, 1, 'Ships'),
    (792, 'Tristan Blueprint', 593, 'Tristan', 1, 4800, 1, 'Ships'),
    (796, 'Punisher Blueprint', 597, 'Punisher', 1, 4800, 1, 'Ships'),
    (32881, 'Venture Blueprint', 32880, 'Venture', 1, 5400, 1, 'Ships'),
    (29249, 'Heron Blueprint', 29248, 'Heron', 1, 4800, 1, 'Ships'),
    -- Destroyers & Cruisers
    (16241, 'Catalyst Blueprint', 16240, 'Catalyst', 1, 9600, 1, 'Ships'),
    (16243, 'Thrasher Blueprint', 16242, 'Thrasher', 1, 9600, 1, 'Ships'),
    (16237, 'Coercer Blueprint', 16236, 'Coercer', 1, 9600, 1, 'Ships'),
    (16239, 'Cormorant Blueprint', 16238, 'Cormorant', 1, 9600, 1, 'Ships'),
    (820, 'Caracal Blueprint', 621, 'Caracal', 1, 18000, 1, 'Ships'),
    (825, 'Vexor Blueprint', 626, 'Vexor', 1, 18000, 1, 'Ships'),
    -- Modules
    (2047, 'Damage Control I Blueprint', 2046, 'Damage Control I', 1, 1800, 1, 'Modules'),
    (3840, 'Large Shield Extender I Blueprint', 3839, 'Large Shield Extender I', 1, 2400, 1, 'Modules'),
    (1199, '1MN Afterburner I Blueprint', 518, '1MN Afterburner I', 1, 1800, 1, 'Modules'),
    (1207, '5MN Microwarpdrive I Blueprint', 526, '5MN Microwarpdrive I', 1, 2400, 1, 'Modules'),
    (4392, 'Medium Shield Booster I Blueprint', 4391, 'Medium Shield Booster I', 1, 2400, 1, 'Modules'),
    (3243, 'Warp Scrambler I Blueprint', 3242, 'Warp Scrambler I', 1, 1800, 1, 'Modules'),
    (1208, 'Stasis Webifier I Blueprint', 527, 'Stasis Webifier I', 1, 1800, 1, 'Modules')
ON CONFLICT (blueprint_type_id) DO NOTHING;

-- 4. Seed Bill of Materials (Base ME 0 quantities per run)
INSERT INTO sde_blueprint_materials (blueprint_type_id, material_type_id, base_quantity) VALUES
    -- Hobgoblin I (2455)
    (2455, 34, 889), (2455, 35, 356), (2455, 36, 78), (2455, 37, 11),
    -- Warrior I (2487)
    (2487, 34, 867), (2487, 35, 344), (2487, 36, 76), (2487, 37, 11),
    -- Hornet I (2465)
    (2465, 34, 911), (2465, 35, 367), (2465, 36, 78), (2465, 37, 11),
    -- Hammerhead I (2184)
    (2184, 34, 3556), (2184, 35, 1333), (2184, 36, 311), (2184, 37, 56), (2184, 38, 11),
    -- Vespa I (2174)
    (2174, 34, 3667), (2174, 35, 1389), (2174, 36, 322), (2174, 37, 56), (2174, 38, 11),
    -- Valkyrie I (2445)
    (2445, 34, 3444), (2445, 35, 1278), (2445, 36, 300), (2445, 37, 56), (2445, 38, 11),
    -- Mining Drone I (10247)
    (10247, 34, 1556), (10247, 35, 622), (10247, 36, 144), (10247, 37, 22),
    -- Antimatter Charge S (1066)
    (1066, 34, 136), (1066, 35, 22), (1066, 36, 11),
    -- Antimatter Charge M (1074)
    (1074, 34, 311), (1074, 35, 56), (1074, 36, 22), (1074, 37, 6),
    -- Scourge Light Missile (1025)
    (1025, 34, 156), (1025, 35, 78), (1025, 36, 17),
    -- Inferno Light Missile (1026)
    (1026, 34, 156), (1026, 35, 78), (1026, 36, 17),
    -- EMP S (999)
    (999, 34, 128), (999, 35, 28), (999, 36, 11),
    -- Phased Plasma S (1001)
    (1001, 34, 128), (1001, 35, 28), (1001, 36, 11),
    -- Rifter (786)
    (786, 34, 32000), (786, 35, 6000), (786, 36, 2500), (786, 37, 500),
    -- Merlin (802)
    (802, 34, 32000), (802, 35, 6000), (802, 36, 2500), (802, 37, 500),
    -- Atron (807)
    (807, 34, 24000), (807, 35, 4500), (807, 36, 1875), (807, 37, 375),
    -- Tristan (792)
    (792, 34, 32000), (792, 35, 6000), (792, 36, 2500), (792, 37, 500),
    -- Punisher (796)
    (796, 34, 32000), (796, 35, 6000), (796, 36, 2500), (796, 37, 500),
    -- Venture (32881)
    (32881, 34, 32000), (32881, 35, 6000), (32881, 36, 2500), (32881, 37, 500),
    -- Heron (29249)
    (29249, 34, 24000), (29249, 35, 4500), (29249, 36, 1875), (29249, 37, 375),
    -- Catalyst (16241)
    (16241, 34, 96000), (16241, 35, 18000), (16241, 36, 7500), (16241, 37, 1500), (16241, 38, 125),
    -- Thrasher (16243)
    (16243, 34, 96000), (16243, 35, 18000), (16243, 36, 7500), (16243, 37, 1500), (16243, 38, 125),
    -- Coercer (16237)
    (16237, 34, 96000), (16237, 35, 18000), (16237, 36, 7500), (16237, 37, 1500), (16237, 38, 125),
    -- Cormorant (16239)
    (16239, 34, 96000), (16239, 35, 18000), (16239, 36, 7500), (16239, 37, 1500), (16239, 38, 125),
    -- Caracal (820)
    (820, 34, 720000), (820, 35, 180000), (820, 36, 56000), (820, 37, 14000), (820, 38, 3500), (820, 39, 1000), (820, 40, 250),
    -- Vexor (825)
    (825, 34, 720000), (825, 35, 180000), (825, 36, 56000), (825, 37, 14000), (825, 38, 3500), (825, 39, 1000), (825, 40, 250),
    -- Damage Control I (2047)
    (2047, 34, 389), (2047, 35, 189), (2047, 36, 56), (2047, 37, 11),
    -- Large Shield Extender I (3840)
    (3840, 34, 1556), (3840, 35, 778), (3840, 36, 222), (3840, 37, 44), (3840, 38, 11),
    -- 1MN Afterburner I (1199)
    (1199, 34, 444), (1199, 35, 222), (1199, 36, 67), (1199, 37, 11),
    -- 5MN Microwarpdrive I (1207)
    (1207, 34, 667), (1207, 35, 333), (1207, 36, 100), (1207, 37, 22),
    -- Medium Shield Booster I (4392)
    (4392, 34, 1111), (4392, 35, 556), (4392, 36, 156), (4392, 37, 33),
    -- Warp Scrambler I (3243)
    (3243, 34, 556), (3243, 35, 278), (3243, 36, 78), (3243, 37, 17),
    -- Stasis Webifier I (1208)
    (1208, 34, 500), (1208, 35, 250), (1208, 36, 67), (1208, 37, 17)
ON CONFLICT (blueprint_type_id, material_type_id) DO NOTHING;
