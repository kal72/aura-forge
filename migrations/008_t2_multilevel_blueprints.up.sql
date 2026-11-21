-- 008_t2_multilevel_blueprints.up.sql
-- Adds starter T2 Drones, T2 Modules, and intermediate sub-component blueprints (2-3 levels deep)
-- for Recursive Industry Tree & Optimal Make-vs-Buy analysis (100% static SDE, 0 ESI calls).

CREATE INDEX IF NOT EXISTS idx_sde_blueprints_product_id ON sde_blueprints (product_type_id);

-- 1. Additional Raw, Moon Reaction, PI & Intermediate Types
INSERT INTO sde_types (type_id, name, group_name, category, volume_m3, portion_size) VALUES
    (11399, 'Morphite', 'Mineral', 'Minerals', 0.01, 1),
    (9848, 'Robotics', 'Specialized Commodities', 'Planetary', 6.0, 1),
    (3828, 'Construction Blocks', 'Refined Commodities', 'Planetary', 1.5, 1),
    (16670, 'Crystalline Carbonide', 'Composite', 'Moon Materials', 0.01, 1),
    (16671, 'Titanium Carbide', 'Composite', 'Moon Materials', 0.01, 1),
    (16672, 'Tungsten Carbide', 'Composite', 'Moon Materials', 0.01, 1),
    (16673, 'Fernite Carbide', 'Composite', 'Moon Materials', 0.01, 1),
    (16678, 'Sylramic Fibers', 'Composite', 'Moon Materials', 0.01, 1),
    (16679, 'Fullerides', 'Composite', 'Moon Materials', 0.01, 1),
    (16680, 'Phenolic Composites', 'Composite', 'Moon Materials', 0.01, 1),
    (16681, 'Nanotransistors', 'Composite', 'Moon Materials', 0.01, 1),
    -- Intermediate Sub-Components (Level 2)
    (11483, 'R.A.M.-Robotics', 'Tool', 'Components', 0.04, 100),
    (11534, 'Photon Microprocessor', 'Construction Components', 'Components', 1.0, 1),
    (11536, 'Oscillator Capacitor Unit', 'Construction Components', 'Components', 1.0, 1),
    (11539, 'Nanoelectrical Microprocessor', 'Construction Components', 'Components', 1.0, 1),
    (11544, 'Crystalline Carbonide Armor Plate', 'Construction Components', 'Components', 1.0, 1),
    -- T2 Drones (Level 1)
    (2456, 'Hobgoblin II', 'Combat Drone', 'Drones', 5.0, 1),
    (2488, 'Warrior II', 'Combat Drone', 'Drones', 5.0, 1),
    (2185, 'Hammerhead II', 'Combat Drone', 'Drones', 10.0, 1),
    -- T2 Modules (Level 1)
    (2048, 'Damage Control II', 'Damage Control', 'Modules', 5.0, 1),
    (5975, '5MN Microwarpdrive II', 'Propulsion Module', 'Modules', 5.0, 1),
    (3244, 'Warp Scrambler II', 'Warp Scrambler', 'Modules', 5.0, 1),
    (3841, 'Large Shield Extender II', 'Shield Extender', 'Modules', 25.0, 1),
    (10858, 'Medium Shield Booster II', 'Shield Booster', 'Modules', 10.0, 1)
ON CONFLICT (type_id) DO NOTHING;

-- 2. Intermediate Sub-Component Blueprints (Level 2 -> Minerals / Moon Composites)
INSERT INTO sde_blueprints (blueprint_type_id, name, product_type_id, product_name, product_qty_per_run, base_time_seconds, tech_level, category) VALUES
    (11485, 'R.A.M.-Robotics Blueprint', 11483, 'R.A.M.-Robotics', 100, 3600, 1, 'Components'),
    (11535, 'Photon Microprocessor Blueprint', 11534, 'Photon Microprocessor', 1, 900, 2, 'Components'),
    (11537, 'Oscillator Capacitor Unit Blueprint', 11536, 'Oscillator Capacitor Unit', 1, 900, 2, 'Components'),
    (11540, 'Nanoelectrical Microprocessor Blueprint', 11539, 'Nanoelectrical Microprocessor', 1, 900, 2, 'Components'),
    (11545, 'Crystalline Carbonide Armor Plate Blueprint', 11544, 'Crystalline Carbonide Armor Plate', 1, 900, 2, 'Components'),
    -- T2 Drones (Level 1 -> T1 Drone + Sub-Components + PI + Morphite)
    (2457, 'Hobgoblin II Blueprint', 2456, 'Hobgoblin II', 1, 5400, 2, 'Drones'),
    (2489, 'Warrior II Blueprint', 2488, 'Warrior II', 1, 5400, 2, 'Drones'),
    (2186, 'Hammerhead II Blueprint', 2185, 'Hammerhead II', 1, 9000, 2, 'Drones'),
    -- T2 Modules (Level 1 -> T1 Module + Sub-Components + PI + Morphite)
    (2049, 'Damage Control II Blueprint', 2048, 'Damage Control II', 1, 7200, 2, 'Modules'),
    (5976, '5MN Microwarpdrive II Blueprint', 5975, '5MN Microwarpdrive II', 1, 7200, 2, 'Modules'),
    (3245, 'Warp Scrambler II Blueprint', 3244, 'Warp Scrambler II', 1, 7200, 2, 'Modules'),
    (3842, 'Large Shield Extender II Blueprint', 3841, 'Large Shield Extender II', 1, 7200, 2, 'Modules'),
    (10859, 'Medium Shield Booster II Blueprint', 10858, 'Medium Shield Booster II', 1, 7200, 2, 'Modules')
ON CONFLICT (blueprint_type_id) DO NOTHING;

-- 3. Sub-Component BOMs (Level 2)
INSERT INTO sde_blueprint_materials (blueprint_type_id, material_type_id, base_quantity) VALUES
    -- R.A.M.-Robotics (100 units/run)
    (11485, 34, 4800), (11485, 35, 1600), (11485, 36, 420), (11485, 37, 110), (11485, 38, 30),
    -- Photon Microprocessor
    (11535, 16679, 17), (11535, 16680, 6), (11535, 16681, 2),
    -- Oscillator Capacitor Unit
    (11537, 16671, 22), (11537, 16679, 11), (11537, 16681, 2),
    -- Nanoelectrical Microprocessor
    (11540, 16670, 18), (11540, 16679, 14), (11540, 16681, 3),
    -- Crystalline Carbonide Armor Plate
    (11545, 16670, 44), (11545, 16678, 11),

    -- 4. T2 Drone BOMs (Multi-Level: requires T1 Drone + Sub-Components + PI + Morphite)
    -- Hobgoblin II
    (2457, 2454, 1), (2457, 11399, 8), (2457, 9848, 2), (2457, 11483, 1), (2457, 11534, 2),
    -- Warrior II
    (2489, 2486, 1), (2489, 11399, 8), (2489, 9848, 2), (2489, 11483, 1), (2489, 11536, 2),
    -- Hammerhead II
    (2186, 2183, 1), (2186, 11399, 16), (2186, 9848, 4), (2186, 11483, 2), (2186, 11534, 4),

    -- 5. T2 Module BOMs (Multi-Level: requires T1 Module + Sub-Components + PI + Morphite)
    -- Damage Control II
    (2049, 2046, 1), (2049, 11399, 14), (2049, 11544, 4), (2049, 11539, 3), (2049, 3828, 6),
    -- 5MN Microwarpdrive II
    (5976, 526, 1), (5976, 11399, 18), (5976, 11536, 5), (5976, 3828, 8),
    -- Warp Scrambler II
    (3245, 3242, 1), (3245, 11399, 12), (3245, 11534, 4), (3245, 11536, 3),
    -- Large Shield Extender II
    (3842, 3839, 1), (3842, 11399, 22), (3842, 11536, 6), (3842, 11539, 4),
    -- Medium Shield Booster II
    (10859, 4391, 1), (10859, 11399, 16), (10859, 11536, 5), (10859, 11534, 4)
ON CONFLICT (blueprint_type_id, material_type_id) DO NOTHING;
